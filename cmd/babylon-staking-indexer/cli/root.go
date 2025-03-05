package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	defaultConfigFileName = "config.yml"
)

var (
	cfgPath string
	rootCmd = &cobra.Command{
		Use: "babylon-staking-indexer",
	}
)

func Setup() error {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	defaultConfigPath := getDefaultConfigFile(homePath, defaultConfigFileName)

	rootCmd.AddCommand(StartServerCmd())
	rootCmd.AddCommand(FillStakerAddrCmd())
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", defaultConfigPath, fmt.Sprintf("config file (default %s)", defaultConfigPath))
	if err := rootCmd.Execute(); err != nil {
		return err
	}

	return nil
}

func getDefaultConfigFile(homePath, filename string) string {
	return filepath.Join(homePath, filename)
}

func GetConfigPath() string {
	return cfgPath
}
