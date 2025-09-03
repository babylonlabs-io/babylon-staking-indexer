package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	dbmodel "github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// BackfillAllowlistCmd backfills BSN allowlists by querying contracts
// Usage: ./babylon-staking-indexer backfill-allowlist --config config.yml [--address <addr> --address <addr> ...] [--dry-run]
func BackfillAllowlistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backfill-allowlist",
		Short: "Backfill BSN allowlists from chain via CosmWasm smart query",
		Run:   backfillAllowlist,
	}

	cmd.Flags().StringArray("address", nil, "Finality contract address to backfill (repeatable). If omitted, processes all BSNs")
	cmd.Flags().Bool("dry-run", false, "Run in simulation mode without making changes")

	return cmd
}

func backfillAllowlist(cmd *cobra.Command, _ []string) {
	cfg, err := config.New(GetConfigPath())
	if err != nil {
		log.Err(err).Msg("Failed to load config")
		os.Exit(1)
	}

	addresses, err := cmd.Flags().GetStringArray("address")
	if err != nil {
		log.Err(err).Msg("Failed to parse address flags")
		os.Exit(1)
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		log.Err(err).Msg("Failed to parse dry-run flag")
		os.Exit(1)
	}

	if err := BackfillAllowlist(cmd.Context(), cfg, addresses, dryRun); err != nil {
		log.Err(err).Msg("Failed to backfill allowlist")
		os.Exit(1)
	}

	os.Exit(0)
}

func BackfillAllowlist(ctx context.Context, cfg *config.Config, addresses []string, dryRun bool) error {
	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	bbnClient, err := bbnclient.NewBBNClient(&cfg.BBN)
	if err != nil {
		return fmt.Errorf("failed to create BBN client: %w", err)
	}

	log := log.Ctx(ctx)

	var targets []*dbmodel.BSN
	if len(addresses) == 0 {
		all, err := dbClient.GetAllBSNs(ctx)
		if err != nil {
			return fmt.Errorf("failed to load BSNs: %w", err)
		}
		targets = all
	} else {
		targets = make([]*dbmodel.BSN, 0, len(addresses))
		for _, addr := range addresses {
			bsn, err := dbClient.GetBSNByAddress(ctx, addr)
			if err != nil {
				log.Error().Err(err).Str("address", addr).Msg("Failed to find BSN by address")
				continue
			}
			targets = append(targets, bsn)
		}
	}

	if len(targets) == 0 {
		log.Info().Msg("No BSNs to backfill")
		return nil
	}

	var errorCount int
	var lastError error
	var successCount int

	// Process BSNs
	for _, bsn := range targets {
		if bsn.RollupMetadata == nil || bsn.RollupMetadata.FinalityContractAddress == "" {
			log.Warn().Str("bsn_id", bsn.ID).Msg("Skipping BSN without finality contract address")
			continue
		}

		const rateLimitDelay = 100 * time.Millisecond
		time.Sleep(rateLimitDelay)

		addr := bsn.RollupMetadata.FinalityContractAddress

		rawAllowlist, err := bbnClient.GetWasmAllowlist(ctx, addr)
		if err != nil {
			log.Error().Err(err).Str("address", addr).Str("bsn_id", bsn.ID).Msg("Failed to fetch allowlist via RPC")
			errorCount++
			lastError = fmt.Errorf("failed to fetch allowlist for %s (BSN %s): %w", addr, bsn.ID, err)
			continue
		}

		allowlist := types.NormalizeAllowlist(rawAllowlist)

		if dryRun {
			log.Info().
				Str("address", addr).
				Str("bsn_id", bsn.ID).
				Str("bsn_name", bsn.Name).
				Int("allowlist_size", len(allowlist)).
				Msg("Dry run: would update BSN allowlist")
			successCount++
			continue
		}

		if err := dbClient.UpdateBSNAllowlist(ctx, addr, allowlist); err != nil {
			log.Error().Err(err).Str("address", addr).Str("bsn_id", bsn.ID).Msg("Failed to persist allowlist")
			errorCount++
			lastError = fmt.Errorf("failed to persist allowlist for %s (BSN %s): %w", addr, bsn.ID, err)
			continue
		}

		log.Info().
			Str("address", addr).
			Str("bsn_id", bsn.ID).
			Str("bsn_name", bsn.Name).
			Int("allowlist_size", len(allowlist)).
			Msg("Successfully updated BSN allowlist")

		successCount++
	}

	if errorCount > 0 {
		log.Error().
			Int("error_count", errorCount).
			Int("success_count", successCount).
			Int("total_count", len(targets)).
			Msg("Some BSNs failed to update during backfill")
		return fmt.Errorf("backfill completed with %d errors out of %d total (last error: %w)", errorCount, len(targets), lastError)
	}

	log.Info().
		Int("processed_count", len(targets)).
		Int("success_count", successCount).
		Msg("Successfully completed backfill with no errors")
	return nil
}
