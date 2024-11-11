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

// StartBbnBlockProcessor initiates the BBN blockchain block processing in a separate goroutine.
// It continuously processes new blocks and their events sequentially, maintaining the chain order.
// If an error occurs, it logs the error and terminates the program.
// The method runs asynchronously to allow non-blocking operation.
func (s *Service) StartBbnBlockProcessor(ctx context.Context) {
	if err := s.processBlocksSequentially(ctx); err != nil {
		log.Fatal().Msgf("BBN block processor exited with error: %v", err)
	}
}

// processBlocksSequentially processes BBN blockchain blocks in sequential order,
// starting from the last processed height up to the latest chain height.
// It extracts events from each block and forwards them to the event processor.
// Returns an error if it fails to get block results or process events.
func (s *Service) processBlocksSequentially(ctx context.Context) *types.Error {
	lastProcessedHeight, dbErr := s.db.GetLastProcessedBbnHeight(ctx)
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
				fmt.Errorf("context cancelled during BBN block processor"),
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
			for i := lastProcessedHeight; i <= uint64(latestHeight); i++ {
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
						if err := s.processEvent(ctx, event); err != nil {
							return err
						}
					}

					if dbErr := s.db.UpdateLastProcessedBbnHeight(ctx, uint64(i)); dbErr != nil {
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
	blockResult, err := s.bbn.GetBlockResultsWithRetry(ctx, &blockHeight)
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
