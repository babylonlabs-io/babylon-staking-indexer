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

	var registeredSpendNotification bool

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
			for {
				select {
				case <-ctx.Done():
					return types.NewError(
						http.StatusInternalServerError,
						types.InternalServiceError,
						fmt.Errorf("context cancelled during block processing"),
					)
				default:
					//delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, "dcecfa8bd68261535a08c27721f68741cefc63a2b5f8e0a6c6204b8df074b722")
					//if dbErr != nil {
					//	return types.NewError(
					//		http.StatusInternalServerError,
					//		types.InternalServiceError,
					//		fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
					//	)
					//}

					//// Emit consumer event if the new state is active
					//err := s.emitUnbondingDelegationEvent(ctx, delegation)
					//if err != nil {
					//	return err
					//}

					// Run this only once to register spend notification for specific staking tx
					if !registeredSpendNotification {
						if err := s.registerStakingSpendNotification(
							ctx,
							"43bca88607bffc1f70b28a8bfa890fe3e6070a59a447f39e9a5f923052e60d4a",
							"02000000000101f0ab4f6a039d2b6bde5b77633b11968b8f8bf734909128c34530834685691acd0200000000fdffffff0250c30000000000002251203f0bb439c0e7a9c446e72a867d712c25c39f92094425e7bdab97a02af38260dd6671e90b000000002251207c0617c9504c1acc02f66408ff2b70b4d3d7f041573d60ccc655a14811fd9e1701408560048eddd8013f4a583457b79298725835489e16d9bbcc46ff101368a4003bb41c68380d6ab88e04a391329d658585f4f6bbd156eccaeb31660a39cab4a26100000000",
							uint32(0),
							uint32(225882),
						); err != nil {
							return err
						}

						log.Info().Msgf("Registered spend notification for staking tx %s", "a380abbd28f463c66ad6866ce0bfbd7d60d0947e141f9e9739e8f8d1e88b811a")

						registeredSpendNotification = true
					}

					// events, err := s.getEventsFromBlock(ctx, int64(i))
					// if err != nil {
					// 	return err
					// }

					// for _, event := range events {
					// 	if err := s.processEvent(ctx, event, int64(i)); err != nil {
					// 		return err
					// 	}
					// }

					// if dbErr := s.db.UpdateLastProcessedBbnHeight(ctx, uint64(i)); dbErr != nil {
					// 	return types.NewError(
					// 		http.StatusInternalServerError,
					// 		types.InternalServiceError,
					// 		fmt.Errorf("failed to update last processed height in database: %w", dbErr),
					// 	)
					// }
					// lastProcessedHeight = i
				}
				//log.Info().Msgf("Processed blocks up to height %d", lastProcessedHeight)
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
) ([]BbnEvent, *types.Error) {
	events := make([]BbnEvent, 0)
	blockResult, err := s.bbn.GetBlockResults(ctx, &blockHeight)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.ClientRequestError,
			fmt.Errorf("failed to get block results: %w", err),
		)
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
