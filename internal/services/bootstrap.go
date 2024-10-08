package services

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog/log"
)

// TODO: To be replaced by the actual values later and moved to a config file
const (
	lastProcessedHeight = 0
	eventProcessorSize  = 5000
	retryInterval       = 10 * time.Second
	maxRetries          = 10
)

// bootstrapBbn handles its own retry logic and runs in a goroutine.
// It will try to bootstrap the BBN blockchain by fetching until the latest block
// height and processing events. If any errors occur during the process,
// it will retry with exponential backoff, up to a maximum of maxRetries.
// The method runs asynchronously to allow non-blocking operation.
func (s *Service) bootstrapBbn(ctx context.Context) {
	go func() {
		bootstrapCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		for retries := 0; retries < maxRetries; retries++ {
			err := s.attemptBootstrap(bootstrapCtx)
			if err != nil {
				log.Err(err).
					Msgf(
						"Failed to bootstrap BBN blockchain, attempt %d/%d",
						retries+1,
						maxRetries,
					)

				// If the retry count reaches maxRetries, log the failure and exit
				if retries == maxRetries-1 {
					log.Fatal().
						Msg(
							"Failed to bootstrap BBN blockchain after max retries, exiting",
						)
				}

				// Exponential backoff
				time.Sleep(retryInterval * time.Duration(retries))
			} else {
				log.Info().Msg("Successfully bootstrapped BBN blockchain")
				break // Exit the loop if successful
			}
		}
	}()
}

// attemptBootstrap tries to bootstrap the BBN blockchain by fetching the latest
// block height and processing the blocks from the last processed height.
// It returns an error if it fails to get the block results or events from the block.
func (s *Service) attemptBootstrap(ctx context.Context) *types.Error {
	latestBbnHeight, err := s.bbn.GetLatestBlockNumber(ctx)
	if err != nil {
		return err
	}
	log.Debug().Msgf("Latest BBN block height: %d", latestBbnHeight)

	// lastProcessedHeight is already synced, so start from the next block
	for i := lastProcessedHeight + 1; i <= latestBbnHeight; i++ {
		events, err := s.getEventsFromBlock(ctx, i)
		if err != nil {
			log.Err(err).Msgf("Failed to get events from block %d", i)
			return err
		}
		for _, event := range events {
			s.bbnEventProcessor <- event
		}
	}
	return nil
}

// getEventsFromBlock fetches the events for a given block by its block height
// and returns them as an array of events. It processes both transaction-level
// events and finalize-block-level events. The events are sourced from the
// /block_result endpoint of the BBN blockchain.
func (s *Service) getEventsFromBlock(
	ctx context.Context, blockHeight int,
) ([]BbnEvent, *types.Error) {
	events := make([]BbnEvent, 0)
	blockResult, err := s.bbn.GetBlockResults(ctx, blockHeight)
	if err != nil {
		return nil, err
	}
	// Append transaction-level events
	for _, txResult := range blockResult.TxResults {
		for _, event := range txResult.Events {
			events = append(events, NewBbnEvent(TxCategory, event))
		}
	}
	// Append finalize-block-level events
	for _, event := range blockResult.FinalizeBlockEvents {
		events = append(events, NewBbnEvent(BlockCategory, event))
	}
	log.Debug().Msgf("Fetched %d events from block %d", len(events), blockHeight)
	return events, nil
}
