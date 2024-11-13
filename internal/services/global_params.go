package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
)

func (s *Service) SyncGlobalParams(ctx context.Context) {
	paramsPoller := poller.NewPoller(
		s.cfg.Poller.ParamPollingInterval,
		s.fetchAndSaveParams,
	)
	go paramsPoller.Start(ctx)
}

func (s *Service) fetchAndSaveParams(ctx context.Context) *types.Error {
	checkpointParams, err := s.bbn.GetCheckpointParams(ctx)
	if err != nil {
		// TODO: Add metrics and replace internal service error with a more specific
		// error code so that the poller can catch and emit the error metrics
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get checkpoint params: %w", err),
		)
	}
	if err := s.db.SaveCheckpointParams(ctx, checkpointParams); err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to save checkpoint params: %w", err),
		)
	}

	allStakingParams, err := s.bbn.GetAllStakingParams(ctx)
	if err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get staking params: %w", err),
		)
	}

	for version, params := range allStakingParams {
		if params == nil {
			return types.NewInternalServiceError(
				fmt.Errorf("nil staking params for version %d", version),
			)
		}
		if err := s.db.SaveStakingParams(ctx, version, params); err != nil {
			return types.NewInternalServiceError(
				fmt.Errorf("failed to save staking params: %w", err),
			)
		}
	}

	return nil
}
