package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog/log"
)

// TODO: To be replaced by the actual values later and moved to a config file
const (
	eventProcessorSize = 5000
)

// BootstrapBbn initiates the BBN blockchain bootstrapping process in a separate goroutine.
// It attempts to bootstrap by processing blocks and events.
// If an error occurs, it logs the error and terminates the program.
// The method runs asynchronously to allow non-blocking operation.
func (s *Service) BootstrapBbn(ctx context.Context) {
	go func() {
		if err := s.attemptBootstrap(ctx); err != nil {
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
					return types.NewError(
						http.StatusInternalServerError,
						types.InternalServiceError,
						fmt.Errorf("context cancelled during block processing"),
					)
				default:
					events, err := s.getEventsFromBlock(ctx, int64(i))
					if err != nil {
						return err
					}
					for _, event := range events {
						s.bbnEventProcessor <- event
					}

					// Update lastProcessedHeight after successful processing
					if dbErr := s.db.UpdateLastProcessedHeight(ctx, i); dbErr != nil {
						return types.NewError(
							http.StatusInternalServerError,
							types.InternalServiceError,
							fmt.Errorf("failed to update last processed height in database: %w", dbErr),
						)
					}
					lastProcessedHeight = i
				}
			}

			log.Info().Msgf("Processed blocks up to height %d", lastProcessedHeight)
		}
	}
}

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
