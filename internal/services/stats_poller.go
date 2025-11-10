package services

import (
	"context"
	"fmt"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
	"github.com/rs/zerolog/log"
)

// StartStatsPoller starts the stats polling service
func (s *Service) StartStatsPoller(ctx context.Context) {
	statsPoller := poller.NewPoller(
		s.cfg.Poller.StatsPollingInterval,
		metrics.RecordPollerDuration("stats", s.calculateAndUpdateStats),
	)
	go statsPoller.Start(ctx)
}

// calculateAndUpdateStats calculates stats using MongoDB aggregation and updates collections
func (s *Service) calculateAndUpdateStats(ctx context.Context) error {
	log := log.Ctx(ctx)

	// Use MongoDB aggregation to calculate stats efficiently without loading all delegations into memory
	startTime := time.Now()
	overallTvl, overallDelegations, fpStats, err := s.db.CalculateActiveStatsAggregated(ctx)
	aggregationDuration := time.Since(startTime)

	log.Debug().
		Dur("aggregation_duration_ms", aggregationDuration).
		Msg("Stats aggregation completed")

	if err != nil {
		return fmt.Errorf("failed to calculate active stats: %w", err)
	}

	// If no delegations exist, skip processing and wait for next poll
	if overallDelegations == 0 {
		log.Debug().Msg("No active delegations found - skipping stats update")
		return nil
	}

	log.Debug().
		Uint64("delegation_count", overallDelegations).
		Int("fp_count", len(fpStats)).
		Msg("Processing active delegations for stats update")

	// Update overall stats
	if err := s.db.UpsertOverallStats(ctx, overallTvl, overallDelegations); err != nil {
		return fmt.Errorf("failed to upsert overall stats: %w", err)
	}

	log.Info().
		Uint64("active_tvl", overallTvl).
		Uint64("active_delegations", overallDelegations).
		Int("fp_count", len(fpStats)).
		Msg("Updated overall stats")

	// Update per-FP stats
	for _, fpStat := range fpStats {
		if err := s.db.UpsertFinalityProviderStats(
			ctx,
			fpStat.FpBtcPkHex,
			fpStat.ActiveTvl,
			fpStat.ActiveDelegations,
		); err != nil {
			log.Error().
				Err(err).
				Str("fp_btc_pk_hex", fpStat.FpBtcPkHex).
				Msg("Failed to upsert finality provider stats")
			return fmt.Errorf("failed to upsert finality provider stats for %s: %w", fpStat.FpBtcPkHex, err)
		}
	}

	log.Debug().
		Int("fp_count", len(fpStats)).
		Msg("Updated finality provider stats")

	// Record metrics
	metrics.RecordActiveTvl(overallTvl)
	metrics.RecordActiveDelegations(int(overallDelegations))

	return nil
}
