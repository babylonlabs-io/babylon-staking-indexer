package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
	"github.com/rs/zerolog/log"
)

func (s *Service) SyncGlobalParams(ctx context.Context) {
	paramsPoller := poller.NewPoller(
		s.cfg.Poller.ParamPollingInterval,
		s.fetchAndSaveParams,
	)
	go paramsPoller.Start(ctx)
}

func (s *Service) fetchAndSaveParams(ctx context.Context) *types.Error {
	log.Debug().Msg("Fetching and saving global parameters")

	checkpointParams, err := s.bbn.GetCheckpointParams(ctx)
	if err != nil {
		// TODO: Add metrics and replace internal service error with a more specific
		// error code so that the poller can catch and emit the error metrics
		log.Error().Err(err).Msg("Failed to get checkpoint params")
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get checkpoint params: %w", err),
		)
	}
	log.Debug().Interface("checkpointParams", checkpointParams).Msg("Retrieved checkpoint params")

	if err := s.db.SaveCheckpointParams(ctx, checkpointParams); err != nil {
		log.Error().Err(err).Msg("Failed to save checkpoint params")
		return types.NewInternalServiceError(
			fmt.Errorf("failed to save checkpoint params: %w", err),
		)
	}
	log.Info().Msg("Successfully saved checkpoint params")

	allStakingParams, err := s.bbn.GetAllStakingParams(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get staking params")
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get staking params: %w", err),
		)
	}
	log.Debug().Interface("allStakingParams", allStakingParams).Msg("Retrieved all staking params")

	for version, params := range allStakingParams {
		if params == nil {
			log.Error().Uint32("version", version).Msg("Nil staking params encountered")
			return types.NewInternalServiceError(
				fmt.Errorf("nil staking params for version %d", version),
			)
		}
		if err := s.db.SaveStakingParams(ctx, version, params); err != nil {
			log.Error().Err(err).Uint32("version", version).Msg("Failed to save staking params")
			return types.NewInternalServiceError(
				fmt.Errorf("failed to save staking params: %w", err),
			)
		}
		log.Info().Uint32("version", version).Msg("Successfully saved staking params")
	}

	log.Info().Msg("Successfully fetched and saved all global parameters")
	return nil
}
