package services

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
)

func (s *Service) SyncGlobalParams(ctx context.Context) {
	paramsPoller := poller.NewPoller(
		s.cfg.Poller.ParamPollingInterval,
		metrics.RecordPollerDuration("fetch_and_save_params", s.fetchAndSaveParams),
	)
	go paramsPoller.Start(ctx)
}

func (s *Service) fetchAndSaveParams(ctx context.Context) error {
	checkpointParams, err := s.bbn.GetCheckpointParams(ctx)
	if err != nil {
		// TODO: Add metrics and replace internal service error with a more specific
		// error code so that the poller can catch and emit the error metrics
		return fmt.Errorf("failed to get checkpoint params: %w", err)
	}
	if err := s.db.SaveCheckpointParams(ctx, checkpointParams); err != nil {
		return fmt.Errorf("failed to save checkpoint params: %w", err)
	}

	var nextVersion uint32
	if s.stakingParamsLatestVersion == 0 {
		// this is the first start of indexer
		nextVersion = 0
	} else {
		// stakingParamsLatestVersion corresponds to latest one stored in the db
		nextVersion = s.stakingParamsLatestVersion + 1
	}

	stakingParams, err := s.bbn.GetStakingParams(ctx, nextVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	versions := slices.Collect(maps.Keys(stakingParams))
	slices.Sort(versions)

	for _, version := range versions {
		params := stakingParams[version]
		if params == nil {
			return fmt.Errorf("nil staking params for version %d", version)
		}

		if err := s.db.SaveStakingParams(ctx, version, params); err != nil && !db.IsDuplicateKeyError(err) {
			return fmt.Errorf("failed to save staking params: %w", err)
		}
		s.stakingParamsLatestVersion = version
	}

	finalityParams, err := s.bbn.GetFinalityParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get finality params: %w", err)
	}
	err = s.db.SaveFinalityParams(ctx, finalityParams)
	if err != nil {
		return fmt.Errorf("failed to save finality provider params: %w", err)
	}

	return nil
}
