package cli

import (
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
	"github.com/spf13/cobra"
	"strconv"
)

func FillStakerAddrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fill-staker-addr [maxHeight]",
		Short: "Fill staker address in delegations till specified height (including)",
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
