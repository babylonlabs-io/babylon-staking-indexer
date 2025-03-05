package cli

import (
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
	"github.com/spf13/cobra"
)

// FillStakerAddrCmd fills staker_babylon_address field in delegations based on previous bbn events
// In order to run it you need to call binary with this command providing maxHeight argument + config flag like  this:
// ./babylon-staking-indexer fill-staker-addr 1000 --config config.yml
func FillStakerAddrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fill-staker-addr [maxHeight]",
		Short: "Fill staker address in delegations till specified height (inclusive)",
		Args:  cobra.ExactArgs(1),
		RunE:  fillStakerAddr,
	}

	return cmd
}

func fillStakerAddr(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	maxHeight, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	cfg, err := config.New(GetConfigPath())
	if err != nil {
		return err
	}

	var dbClient db.DbInterface
	dbClient, err = db.New(ctx, cfg.Db)
	if err != nil {
		return err
	}

	bbnClient := bbnclient.NewBBNClient(&cfg.BBN)
	srv := services.NewService(cfg, dbClient, nil, nil, bbnClient, nil)

	return srv.FillStakerAddr(ctx, maxHeight)
}
