package cli

import (
	"fmt"
	"os"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func FillMaxFinalityProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "fill-max-fp",
		Run: fillMaxFinalityProviders,
	}

	cmd.Flags().Bool("dry-run", false, "Run in simulation mode without making changes")

	return cmd
}

func fillMaxFinalityProviders(cmd *cobra.Command, args []string) {
	err := fillMaxFinalityProvidersE(cmd, args)
	// because of current architecture we need to stop execution of the program
	// otherwise existing main logic will be called
	if err != nil {
		log.Err(err).Msg("Failed to fill staker address")
		os.Exit(1)
	}

	os.Exit(0)
}

func fillMaxFinalityProvidersE(cmd *cobra.Command, _ []string) error {
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

	var (
		version           = uint32(0)
		lastStakingParams *bbnclient.StakingParams
	)
	for {
		var err error
		lastStakingParams, err = dbClient.GetStakingParams(ctx, version)
		if err != nil {
			if db.IsNotFoundError(err) {
				break
			}
			return err
		}

		version++
	}

	fmt.Printf("Found last staking params version -> %d with max_finality_providers value = %d\n", version, lastStakingParams.MaxFinalityProviders)

	if lastStakingParams.MaxFinalityProviders != 0 {
		fmt.Println("Property max_finality_providers already set. Exiting")
		return nil
	}

	bbnClient, err := bbnclient.NewBBNClient(&cfg.BBN)
	if err != nil {
		return err
	}

	allStakingParams, err := bbnClient.GetAllStakingParams(ctx)
	if err != nil {
		return err
	}
	maxFinalityProviders := allStakingParams[version].MaxFinalityProviders

	fmt.Printf("Updating staking params version %d with max_finality_providers value = %d\n", version, maxFinalityProviders)
	if dryRun {
		return nil
	}

	return dbClient.UpdateStakingParamMaxFinalityProviders(ctx, version, maxFinalityProviders)
}
