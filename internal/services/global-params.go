package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
)

// CHECKPOINT_PARAMS_VERSION is the version of the checkpoint params
// the value is hardcoded to 0 as the checkpoint params are not expected to change
// However, we keep the versioning in place for future compatibility and
// maintain the same pattern as other global params
const (
	CHECKPOINT_PARAMS_VERSION = 0
	CHECKPOINT_PARAMS_TYPE    = "CHECKPOINT"
	STAKING_PARAMS_TYPE       = "STAKING"
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
	if err := s.db.SaveGlobalParams(ctx, &model.GolablParamDocument{
		Type:    CHECKPOINT_PARAMS_TYPE,
		Version: CHECKPOINT_PARAMS_VERSION,
		Params:  checkpointParams,
	}); err != nil {
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
		if err := s.db.SaveGlobalParams(ctx, &model.GolablParamDocument{
			Type:    STAKING_PARAMS_TYPE,
			Version: version,
			Params:  params,
		}); err != nil {
			return types.NewInternalServiceError(
				fmt.Errorf("failed to save staking params: %w", err),
			)
		}
	}
	return nil
}
