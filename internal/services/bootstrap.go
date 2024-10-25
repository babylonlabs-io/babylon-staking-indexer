package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog/log"
)

// TODO: To be replaced by the actual values later and moved to a config file
const (
	lastProcessedHeight = int64(0)
	eventProcessorSize  = 5000
	retryInterval       = 10 * time.Second
)

// BootstrapBbn initiates the BBN blockchain bootstrapping process in a separate goroutine.
// It attempts to bootstrap by processing blocks and events.
// If an error occurs, it logs the error and terminates the program.
// The method runs asynchronously to allow non-blocking operation.
func (s *Service) BootstrapBbn(ctx context.Context) {
	go func() {
		err := s.attemptBootstrap(ctx)
		if err != nil {
			log.Fatal().Msgf("BBN bootstrap process exited with error: %v", err)
		}
	}()
}

// attemptBootstrap tries to bootstrap the BBN blockchain by fetching the latest
// block height and processing the blocks from the last processed height.
// It returns an error if it fails to get the block results or events from the block.
func (s *Service) attemptBootstrap(ctx context.Context) *types.Error {
	lastProcessedHeight, dbErr := s.db.GetLastProcessedHeight(ctx)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get last processed height: %w", dbErr),
		)
	}

	for {
		select {
		case <-ctx.Done():
			return types.NewError(
				http.StatusInternalServerError,
				types.InternalServiceError,
				fmt.Errorf("context cancelled during bootstrap"),
			)

		case latestHeight := <-s.latestHeightChan:
			log.Info().
				Uint64("last_processed_height", lastProcessedHeight).
				Int64("latest_height", latestHeight).
				Msg("Received new block height")

			// Process blocks from lastProcessedHeight + 1 to latestHeight
			for i := lastProcessedHeight + 1; i <= uint64(latestHeight); i++ {
				select {
				case <-ctx.Done():
					return types.NewError(
						http.StatusInternalServerError,
						types.InternalServiceError,
						fmt.Errorf("context cancelled during block processing"),
					)
				default:
					events, err := s.getEventsFromBlock(ctx, int64(i))
					if err != nil {
						log.Err(err).Msgf("Failed to get events from block %d", i)
						return err
					}
					for _, event := range events {
						s.bbnEventProcessor <- event
					}

					// Update lastProcessedHeight after successful processing
					if err := s.db.UpdateLastProcessedHeight(ctx, i); err != nil {
						log.Err(err).Msg("Failed to update last processed height")
					}
					lastProcessedHeight = i
				}
			}

			log.Info().Msgf("Processed blocks up to height %d", lastProcessedHeight)
		}
	}
}

//func (s *Service) attemptBootstrap(ctx context.Context) *types.Error {
//	latestBbnHeight, err := s.bbn.GetLatestBlockNumber(ctx)
//	if err != nil {
//		return err
//	}
//
//	err2 := s.bbn.Start()
//	if err2 != nil {
//		return types.NewError(
//			http.StatusInternalServerError,
//			types.InternalServiceError,
//			fmt.Errorf("failed to start BBN client"),
//		)
//	}
//
//	log.Debug().Msgf("BBN client is running: %v", s.bbn.IsRunning())
//	log.Debug().Msgf("Latest BBN block height: %d", latestBbnHeight)
//
//	// lastProcessedHeight is already synced, so start from the next block
//	for i := lastProcessedHeight + 1; i <= latestBbnHeight; i++ {
//		events, err := s.getEventsFromBlock(ctx, i)
//		if err != nil {
//			log.Err(err).Msgf("Failed to get events from block %d", i)
//			return err
//		}
//		for _, event := range events {
//			s.bbnEventProcessor <- event
//		}
//	}
//	return nil
//}

// getEventsFromBlock fetches the events for a given block by its block height
// and returns them as an array of events. It processes both transaction-level
// events and finalize-block-level events. The events are sourced from the
// /block_result endpoint of the BBN blockchain.
func (s *Service) getEventsFromBlock(
	ctx context.Context, blockHeight int64,
) ([]BbnEvent, *types.Error) {
	events := make([]BbnEvent, 0)
	blockResult, err := s.bbn.GetBlockResults(ctx, blockHeight)
	if err != nil {
		return nil, err
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
	log.Debug().Msgf("Fetched %d events from block %d", len(events), blockHeight)
	return events, nil
}
