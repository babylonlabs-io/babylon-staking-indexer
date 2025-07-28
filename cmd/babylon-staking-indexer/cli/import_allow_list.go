package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func ImportAllowListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-allow-list",
		Short: "Import allow-list",
		Run:   importAllowList,
	}

	return cmd
}

func importAllowList(cmd *cobra.Command, args []string) {
	err := importAllowListE(cmd, args)
	// because of current architecture we need to stop execution of the program
	// otherwise existing main logic will be called
	if err != nil {
		log.Err(err).Msg("Failed to update overall stats")
		os.Exit(1)
	}

	os.Exit(0)
}

func importAllowListE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if len(args) == 0 {
		return fmt.Errorf("empty allow-list file")
	}

	cfg, err := config.New(GetConfigPath())
	if err != nil {
		return err
	}

	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		return err
	}

	filename := args[0]
	fd, err := os.Open(filename)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(fd)
	for sc.Scan() {
		stakingTxHash := sc.Text()
		stakingTxHash = strings.TrimSpace(stakingTxHash)

		err = dbClient.SetBTCDelegationCanExpand(ctx, stakingTxHash)
		if db.IsNotFoundError(err) {
			fmt.Printf("Error: delegation %q hasn't been found\n", stakingTxHash)
		} else if err != nil {
			return err
		}

		fmt.Printf("Delegation %q was updated\n", stakingTxHash)
	}

	return sc.Err()
}
