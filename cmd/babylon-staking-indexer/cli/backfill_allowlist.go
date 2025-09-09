package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	dbmodel "github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const (
	allowlistPreviewLimit = 3
	dirPermissions        = 0o755
	filePermissions       = 0o666
)

// setupFileLogging creates a file logger that writes to timestamped log files
func setupFileLogging(dryRun bool) (*os.File, error) {
	// Create logs directory if it doesn't exist
	logsDir := "logs"

	if err := os.MkdirAll(logsDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create timestamped filename
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	mode := "live"
	if dryRun {
		mode = "dry-run"
	}
	filename := fmt.Sprintf("backfill-allowlist-%s-%s.log", mode, timestamp)
	logPath := filepath.Join(logsDir, filename)

	// Create and open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %s: %w", logPath, err)
	}

	// Configure zerolog to write to both console and file
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	multi := zerolog.MultiLevelWriter(consoleWriter, logFile)
	logger := zerolog.New(multi).With().Timestamp().Logger()

	log.Logger = logger

	log.Info().
		Str("log_file", logPath).
		Str("mode", mode).
		Msg("File logging initialized")

	return logFile, nil
}

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
	// Parse flags before setting up logging
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		log.Err(err).Msg("Failed to parse dry-run flag")
		os.Exit(1)
	}

	// Setup file logging first
	logFile, err := setupFileLogging(dryRun)
	if err != nil {
		log.Err(err).Msg("Failed to setup file logging")
		os.Exit(1)
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	log.Info().
		Bool("dry_run", dryRun).
		Str("command", "backfill-allowlist").
		Msg("Starting backfill allowlist operation")

	cfg, err := config.New(GetConfigPath())
	if err != nil {
		log.Err(err).Str("config_path", GetConfigPath()).Msg("Failed to load config")
		os.Exit(1)
	}

	addresses, err := cmd.Flags().GetStringArray("address")
	if err != nil {
		log.Err(err).Msg("Failed to parse address flags")
		os.Exit(1)
	}

	log.Info().
		Str("config_path", GetConfigPath()).
		Strs("target_addresses", addresses).
		Bool("all_bsns", len(addresses) == 0).
		Msg("Configuration loaded successfully")

	if err := BackfillAllowlist(cmd.Context(), cfg, addresses, dryRun); err != nil {
		log.Err(err).Msg("Failed to backfill allowlist")
		os.Exit(1)
	}

	log.Info().Msg("Backfill allowlist operation completed successfully")
	os.Exit(0)
}

func BackfillAllowlist(ctx context.Context, cfg *config.Config, addresses []string, dryRun bool) error {
	startTime := time.Now()

	logger := log.Logger

	logger.Info().
		Str("operation", "BackfillAllowlist").
		Bool("dry_run", dryRun).
		Strs("target_addresses", addresses).
		Str("db_address", cfg.Db.Address).
		Str("bbn_rpc", cfg.BBN.RPCAddr).
		Msg("=== STARTING BACKFILL ALLOWLIST OPERATION ===")

	// Step 1: Database Connection
	logger.Info().Str("step", "1").Msg("Connecting to database...")
	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		logger.Error().Err(err).
			Str("db_address", cfg.Db.Address).
			Str("db_name", cfg.Db.DbName).
			Msg("Database connection failed")
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	logger.Info().Str("step", "1").Msg("Database connection successful")

	// Step 2: BBN Client Setup
	logger.Info().Str("step", "2").Msg("Setting up BBN client...")
	bbnClient, err := bbnclient.NewBBNClient(&cfg.BBN)
	if err != nil {
		logger.Error().Err(err).
			Str("rpc_addr", cfg.BBN.RPCAddr).
			Str("lcd_addr", cfg.BBN.LCDAddr).
			Msg("BBN client setup failed")
		return fmt.Errorf("failed to create BBN client: %w", err)
	}
	logger.Info().
		Str("step", "2").
		Str("rpc_addr", cfg.BBN.RPCAddr).
		Str("lcd_addr", cfg.BBN.LCDAddr).
		Dur("timeout", cfg.BBN.Timeout).
		Msg("BBN client setup successful")

	// Step 3: BSN Discovery
	logger.Info().Str("step", "3").Msg("Discovering target BSNs...")
	var targets []*dbmodel.BSN

	if len(addresses) == 0 {
		logger.Info().Msg("Fetching all BSNs from database")
		all, err := dbClient.GetAllBSNs(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to load all BSNs from database")
			return fmt.Errorf("failed to load BSNs: %w", err)
		}
		targets = all
		logger.Info().
			Int("total_bsns_found", len(all)).
			Msg("Successfully loaded all BSNs from database")
	} else {
		logger.Info().
			Strs("target_addresses", addresses).
			Msg("Looking up BSNs by specific addresses")
		targets = make([]*dbmodel.BSN, 0, len(addresses))

		for i, addr := range addresses {
			bsn, err := dbClient.GetBSNByAddress(ctx, addr)
			if err != nil {
				logger.Error().Err(err).
					Str("address", addr).
					Int("address_index", i+1).
					Msg("Failed to find BSN by address")
				continue
			}
			targets = append(targets, bsn)
		}
	}

	// Step 4: BSN Filtering and Validation
	logger.Info().
		Str("step", "4").
		Int("total_bsns_found", len(targets)).
		Msg("Filtering and validating BSNs...")

	validTargets := make([]*dbmodel.BSN, 0, len(targets))
	skippedCount := 0

	for _, bsn := range targets {
		if bsn.RollupMetadata == nil {
			skippedCount++
			continue
		}

		if bsn.RollupMetadata.FinalityContractAddress == "" {
			skippedCount++
			continue
		}

		validTargets = append(validTargets, bsn)
	}

	logger.Info().
		Str("step", "4").
		Int("total_bsns_found", len(targets)).
		Int("valid_bsns", len(validTargets)).
		Int("skipped_bsns", skippedCount).
		Msg("BSN filtering completed")

	if len(validTargets) == 0 {
		logger.Warn().
			Int("total_bsns_checked", len(targets)).
			Int("skipped_count", skippedCount).
			Msg("NO VALID BSNs TO PROCESS - Operation complete")

		// Provide debugging help
		if len(targets) == 0 {
			logger.Info().Msg("Debug: No BSNs found in database. Check if BSNs are properly registered.")
		} else {
			logger.Info().
				Int("total_found", len(targets)).
				Msg("Debug: BSNs found but none have valid finality contract addresses. Check BSN rollup metadata.")
		}
		return nil
	}

	// Step 5: Processing Phase
	logger.Info().
		Str("step", "5").
		Int("valid_targets", len(validTargets)).
		Bool("dry_run", dryRun).
		Msg("=== STARTING BSN PROCESSING PHASE ===")

	var errorCount int
	var lastError error
	var successCount int

	for i, bsn := range validTargets {
		processingIndex := i + 1
		logger.Info().
			Int("processing_index", processingIndex).
			Int("total_to_process", len(validTargets)).
			Str("bsn_id", bsn.ID).
			Str("bsn_name", bsn.Name).
			Str("contract_address", bsn.RollupMetadata.FinalityContractAddress).
			Msg("Processing BSN")

		// Rate limiting
		if i > 0 {
			const rateLimitDelay = 100 * time.Millisecond
			logger.Debug().
				Dur("delay", rateLimitDelay).
				Msg("Rate limiting delay")
			time.Sleep(rateLimitDelay)
		}

		addr := bsn.RollupMetadata.FinalityContractAddress

		// Step 5.1: Fetch allowlist from chain
		logger.Info().
			Int("processing_index", processingIndex).
			Str("step", "5.1").
			Str("contract_address", addr).
			Msg("Fetching allowlist from chain")

		rawAllowlist, err := bbnClient.GetWasmAllowlist(ctx, addr)
		if err != nil {
			logger.Error().Err(err).
				Int("processing_index", processingIndex).
				Str("address", addr).
				Str("bsn_id", bsn.ID).
				Str("bsn_name", bsn.Name).
				Msg("Failed to fetch allowlist from chain")
			errorCount++
			lastError = fmt.Errorf("failed to fetch allowlist for %s (BSN %s): %w", addr, bsn.ID, err)
			continue
		}

		// Step 5.2: Normalize allowlist
		logger.Debug().
			Int("processing_index", processingIndex).
			Str("step", "5.2").
			Interface("raw_allowlist", rawAllowlist).
			Msg("Normalizing allowlist")

		allowlist := types.NormalizeAllowlist(rawAllowlist)

		logger.Info().
			Int("processing_index", processingIndex).
			Str("step", "5.2").
			Int("raw_size", len(rawAllowlist)).
			Int("normalized_size", len(allowlist)).
			Msg("Allowlist normalized")

		// Enhanced dry-run or live processing
		if dryRun {
			logger.Info().
				Int("processing_index", processingIndex).
				Str("mode", "DRY-RUN").
				Str("address", addr).
				Str("bsn_id", bsn.ID).
				Str("bsn_name", bsn.Name).
				Int("allowlist_size", len(allowlist)).
				Strs("allowlist_preview", func() []string {
					if len(allowlist) <= 5 {
						return allowlist
					}
					return append(allowlist[:allowlistPreviewLimit], fmt.Sprintf("... and %d more", len(allowlist)-allowlistPreviewLimit))
				}()).
				Msg("DRY-RUN: Would update BSN allowlist")
			successCount++
		} else {
			// Step 5.3: Update database
			logger.Info().
				Int("processing_index", processingIndex).
				Str("step", "5.3").
				Str("mode", "LIVE").
				Int("allowlist_size", len(allowlist)).
				Msg("Updating database")

			if err := dbClient.UpdateBSNAllowlist(ctx, addr, allowlist); err != nil {
				logger.Error().Err(err).
					Int("processing_index", processingIndex).
					Str("address", addr).
					Str("bsn_id", bsn.ID).
					Str("bsn_name", bsn.Name).
					Msg("Failed to persist allowlist to database")
				errorCount++
				lastError = fmt.Errorf("failed to persist allowlist for %s (BSN %s): %w", addr, bsn.ID, err)
				continue
			}

			logger.Info().
				Int("processing_index", processingIndex).
				Str("address", addr).
				Str("bsn_id", bsn.ID).
				Str("bsn_name", bsn.Name).
				Int("allowlist_size", len(allowlist)).
				Msg("Successfully updated BSN allowlist in database")
			successCount++
		}

		logger.Info().
			Int("processing_index", processingIndex).
			Str("bsn_id", bsn.ID).
			Int("completed", successCount).
			Int("remaining", len(validTargets)-processingIndex).
			Msg("BSN processing completed")
	}

	// Step 6: Final Summary
	duration := time.Since(startTime)

	if errorCount > 0 {
		logger.Error().
			Int("error_count", errorCount).
			Int("success_count", successCount).
			Int("total_processed", len(validTargets)).
			Dur("total_duration", duration).
			Msg("BACKFILL COMPLETED WITH ERRORS")
		return fmt.Errorf("backfill completed with %d errors out of %d total (last error: %w)", errorCount, len(validTargets), lastError)
	}

	logger.Info().
		Int("total_bsns_found", len(targets)).
		Int("valid_bsns", len(validTargets)).
		Int("processed_count", len(validTargets)).
		Int("success_count", successCount).
		Dur("total_duration", duration).
		Bool("dry_run", dryRun).
		Msg("BACKFILL COMPLETED SUCCESSFULLY WITH NO ERRORS")

	return nil
}
