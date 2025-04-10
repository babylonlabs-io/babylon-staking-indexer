package cli

import (
	"fmt"
	"os"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// FillStakerAddrCmd fills staker_babylon_address field in delegations based on previous bbn events
// In order to run it you need to call binary with this command + config flag like this:
// ./babylon-staking-indexer fill-staker-addr --config config.yml
func FillStakerAddrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fill-staker-addr",
		Short: "Fill staker address in delegations with empty staker_babylon_address field",
		Run:   fillStakerAddr,
	}

	cmd.Flags().Bool("dry-run", false, "Run in simulation mode without making changes")
	cmd.Flags().Int("workers", 0, "Number of workers to process records")

	return cmd
}

func fillStakerAddr(cmd *cobra.Command, args []string) {
	err := fillStakerAddrE(cmd, args)
	// because of current architecture we need to stop execution of the program
	// otherwise existing main logic will be called
	if err != nil {
		log.Err(err).Msg("Failed to fill staker address")
		os.Exit(1)
	}

	os.Exit(0)
}

func fillStakerAddrE(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.New(GetConfigPath())
	if err != nil {
		return err
	}

	var dbClient db.DbInterface
	dbClient, err = db.New(ctx, cfg.Db)
	if err != nil {
		return err
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}

	numWorkers, err := cmd.Flags().GetInt("workers")
	if err != nil {
		return err
	}
	if numWorkers <= 0 {
		return fmt.Errorf("number of workers must be greater than 0")
	}

	bbnClient, err := bbnclient.NewBBNClient(&cfg.BBN)
	if err != nil {
		return err
	}
	srv := services.NewService(cfg, dbClient, nil, nil, bbnClient, nil)

	return srv.FillStakerAddr(ctx, numWorkers, dryRun)
}
