package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/babylonlabs-io/babylon-staking-indexer/pkg"
	"github.com/rs/zerolog/log"
	"github.com/sourcegraph/conc"
)

// TODO: To be replaced by the actual values later and moved to a config file
const eventProcessorSize = 100

// StartBbnBlockProcessor initiates the BBN blockchain block processing in a separate goroutine.
// It continuously processes new blocks and their events sequentially, maintaining the chain order.
// If an error occurs, it logs the error and terminates the program.
// The method runs asynchronously to allow non-blocking operation.
func (s *Service) StartBbnBlockProcessor(ctx context.Context) error {
	if err := s.processBlocksSequentially(ctx); err != nil {
		return fmt.Errorf("BBN block processor exited with error: %w", err)
	}

	return nil
}

// FillStakerAddr is temporary method to backfill staker_addr data in the database.
func (s *Service) FillStakerAddr(ctx context.Context, numWorkers int, dryRun bool) error {
	records, err := s.db.GetDelegationsWithEmptyStakerAddress(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	hashes := make(chan string, 100)

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		defer close(hashes)

		for _, record := range records {
			select {
			case hashes <- record.StakingTxHashHex:
				// do nothing
			case <-ctx.Done():
				return
			}
		}
	})

	var (
		errs    []error
		errorMx sync.Mutex
	)
	addError := func(err error) {
		errorMx.Lock()
		errs = append(errs, err)
		errorMx.Unlock()
		// in case of error cancel outer context
		cancel()
	}

	for range numWorkers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case stakingTxHashHex, ok := <-hashes:
					if !ok {
						return
					}

					bbnAddress, err := s.bbn.BabylonStakerAddress(ctx, stakingTxHashHex)
					if err != nil {
						addError(err)
						return
					}

					if bbnAddress == "" {
						addError(fmt.Errorf("empty staker address for tx %s", stakingTxHashHex))
						return
					}

					// double check
					err = pkg.ValidateBabylonAddress(bbnAddress)
					if err != nil {
						addError(err)
						return
					}

					if !dryRun {
						if dbErr := s.db.UpdateDelegationStakerBabylonAddress(
							ctx, stakingTxHashHex, bbnAddress,
						); dbErr != nil {
							addError(fmt.Errorf("failed to updated %s delegation staker addr: %w", stakingTxHashHex, dbErr))
							return
						}
						log.Info().Msgf("Record %s updated with address %s", stakingTxHashHex, bbnAddress)
					} else {
						log.Info().Msgf("Dry run: would update record %s with address %s", stakingTxHashHex, bbnAddress)
					}
				}
			}
		})
	}
	wg.Wait()

	return errors.Join(errs...)
}

// processBlocksSequentially processes BBN blockchain blocks in sequential order,
// starting from the last processed height up to the latest chain height.
// It extracts events from each block and forwards them to the event processor.
// Returns an error if it fails to get block results or process events.
func (s *Service) processBlocksSequentially(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)

	lastProcessedHeight, dbErr := s.db.GetLastProcessedBbnHeight(ctx)
	if dbErr != nil {
		return fmt.Errorf("failed to get last processed height: %w", dbErr)
	}
	log := log.Ctx(ctx)

	type blockEvents struct {
		blockHeight int64
		events      []BbnEvent
	}
	blockEventsCh := make(chan blockEvents, eventProcessorSize)
	orderedBlockEventsCh := make(chan blockEvents)
	blocks := make(chan int64, eventProcessorSize)

	var wg, wg2 conc.WaitGroup
	expectedHeight := int64(lastProcessedHeight + 1)
	wg.Go(func() {
		defer close(orderedBlockEventsCh)

		// blockNumber -> blockEvents
		queue := make(map[int64]blockEvents)
		for block := range blockEventsCh {
			fmt.Printf("Block height %d (expected %d, queue len %d)\n", block.blockHeight, expectedHeight, len(queue))
			if expectedHeight != block.blockHeight {
				queue[block.blockHeight] = block
				continue
			}

			orderedBlockEventsCh <- block
			expectedHeight++

			// process pending blocks in the queue
			for len(queue) > 0 {
				if _, ok := queue[expectedHeight]; !ok {
					break
				}

				block = queue[expectedHeight]
				delete(queue, expectedHeight)

				orderedBlockEventsCh <- block
				expectedHeight++
			}
		}
	})

	for range 3 {
		wg2.Go(func() {
			for block := range blocks {
				events, err := s.getEventsFromBlock(ctx, block)
				if err != nil {
					cancel(err)
					return
				}

				item := blockEvents{
					blockHeight: block,
					events:      events,
				}

				fmt.Println("Sending block", block, "with events", len(events))
				blockEventsCh <- item
			}
		})
	}

	wg.Go(func() {
		// this goroutine exits in 2 cases:
		// 1. blockEventsCh is closed which means parent goroutine is done (it will wait this one to finish processing though)
		// 2. there is an error in this goroutine during processing one of the blocks (note that cause will be available through context)
		for item := range orderedBlockEventsCh {
			for _, event := range item.events {
				err := s.processEvent(ctx, event, item.blockHeight)
				if err != nil {
					cancel(err)
					return
				}
			}

			dbErr := s.db.UpdateLastProcessedBbnHeight(ctx, uint64(item.blockHeight))
			if dbErr != nil {
				cancel(fmt.Errorf("failed to update last processed height in database: %w", dbErr))
				return
			}
		}
	})
	defer func() {
		close(blocks)
		wg2.Wait()
		close(blockEventsCh)
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			ctxErr := context.Cause(ctx)
			return fmt.Errorf("context cancelled during BBN block processor: %w", ctxErr)

		case height := <-s.latestHeightChan:
			// Drain channel to get the most recent height
			latestHeight := s.getLatestHeight(height)

			log.Debug().
				Uint64("last_processed_height", lastProcessedHeight).
				Int64("latest_height", latestHeight).
				Msg("Received new block height")

			if uint64(latestHeight) <= lastProcessedHeight {
				continue
			}

			// Process blocks from lastProcessedHeight + 1 to latestHeight
			for i := lastProcessedHeight + 1; i <= uint64(latestHeight); i++ {
				select {
				case <-ctx.Done():
					ctxErr := context.Cause(ctx)
					return fmt.Errorf("context cancelled during BBN block processor: %w", ctxErr)
				case blocks <- int64(i):
					fmt.Println("block sent", i)
				}
			}
		}
	}
}

// getEventsFromBlock fetches the events for a given block by its block height
// and returns them as an array of events. It processes both transaction-level
// events and finalize-block-level events. The events are sourced from the
// /block_result endpoint of the BBN blockchain.
func (s *Service) getEventsFromBlock(
	ctx context.Context, blockHeight int64,
) ([]BbnEvent, error) {
	events := make([]BbnEvent, 0)
	blockResult, err := s.bbn.GetBlockResults(ctx, &blockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to get block results: %w", err)
	}
	// Append transaction-level events
	for _, txResult := range blockResult.TxsResults {
		for _, event := range txResult.Events {
			events = append(events, NewBbnEvent(TxCategory, event))
		}
	}
	// Append finalize-block-level events
	for _, event := range blockResult.FinalizeBlockEvents {
		events = append(events, NewBbnEvent(BlockCategory, event))
	}
	log.Ctx(ctx).Debug().Msgf("Fetched %d events from block %d", len(events), blockHeight)
	return events, nil
}

func (s *Service) getLatestHeight(initialHeight int64) int64 {
	latestHeight := initialHeight
	// Drain the channel to get the most recent height
	for {
		select {
		case newHeight := <-s.latestHeightChan:
			latestHeight = newHeight
		default:
			// No more values in channel, return the latest height
			return latestHeight
		}
	}
}
