package services

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
	"github.com/rs/zerolog/log"
)

func (s *Service) SyncGlobalParams(ctx context.Context) {
	paramsPoller := poller.NewPoller(
		s.cfg.Poller.ParamPollingInterval,
		metrics.RecordPollerDuration("fetch_and_save_params", s.fetchAndSaveParams),
	)
	go paramsPoller.Start(ctx)
	go s.fetchAndSaveNetworkInfo(ctx)
}

func (s *Service) fetchAndSaveNetworkInfo(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	log := log.Ctx(ctx)

	for range ticker.C {
		chainID, err := s.bbn.GetChainID(ctx)
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch chain ID")
			continue
		}

		doc := &model.NetworkInfo{
			ChainID: chainID,
		}
		err = s.db.UpsertNetworkInfo(ctx, doc)
		if err != nil {
			log.Error().Err(err).Msg("failed to upsert network info")
			continue
		}

		// successfully stored network info
		break
	}
}

// updateMaxFinalityProviders updates params.MaxFinalityProviders in staking params collection for a specific version
func (s *Service) updateMaxFinalityProviders(ctx context.Context, version uint32) {
	dbParams, err := s.db.GetStakingParams(ctx, version)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("updateMaxFinalityProviders: failed to fetch staking params")
		return
	}

	if dbParams.MaxFinalityProviders != 0 {
		// already updated
		return
	}

	bbnParams, err := s.bbn.GetAllStakingParams(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("updateMaxFinalityProviders: failed to get bbn staking params")
		return
	}

	bbnParamsForVersion := bbnParams[version]
	if bbnParamsForVersion == nil {
		log.Ctx(ctx).Error().Msg("updateMaxFinalityProviders: maxFinalityProviders is nil")
		return
	}

	err = s.db.UpdateStakingParamMaxFinalityProviders(ctx, version, bbnParamsForVersion.MaxFinalityProviders)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("updateMaxFinalityProviders: failed to update maxFinalityProviders")
	}
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

	if !s.lastStakingParamsUpdated {
		s.updateMaxFinalityProviders(ctx, s.stakingParamsLatestVersion)
		s.lastStakingParamsUpdated = true
	}

	return nil
}
