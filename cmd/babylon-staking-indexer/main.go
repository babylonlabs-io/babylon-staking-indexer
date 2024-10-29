package main

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/cmd/babylon-staking-indexer/cli"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Debug().Msg("failed to load .env file")
	}
}

func main() {
	ctx := context.Background()

	// setup cli commands and flags
	if err := cli.Setup(); err != nil {
		panic(err)
	}

	// load config
	cfgPath := cli.GetConfigPath()
	cfg, err := config.New(cfgPath)
	if err != nil {
		panic(fmt.Sprintf("error while loading config file: %s", cfgPath))
	}

	// create new db client
	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		panic(fmt.Errorf("error while creating db client: %w", err))
	}

	btcClient, err := btcclient.NewBtcClient(&cfg.BTC)
	if err != nil {
		panic(fmt.Errorf("error while creating btc client: %w", err))
	}
	bbnClient := bbnclient.NewBbnClient(&cfg.Bbn)

	qm, err := queue.NewQueueManager(&cfg.Queue)
	if err != nil {
		panic(fmt.Errorf("error while creating queue manager: %w", err))
	}

	btcNotifier, err := btcclient.NewNodeBackendWithParams(cfg.BTC)
	if err != nil {
		panic(fmt.Errorf("error while creating btc notifier: %w", err))
	}

	service := services.NewService(cfg, dbClient, btcClient, btcNotifier, bbnClient, qm)
	if err != nil {
		panic(fmt.Errorf("error while creating service: %w", err))
	}

	if err := btcNotifier.Start(); err != nil {
		panic(fmt.Errorf("failed to start btc chain notifier: %w", err))
	}

	// initialize metrics with the metrics port from config
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	service.StartIndexerSync(ctx)
}
