package main

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/cmd/babylon-staking-indexer/cli"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/client/btcclient"
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
		log.Fatal().Err(err).Msg("error while setting up cli")
	}

	// load config
	cfgPath := cli.GetConfigPath()
	cfg, err := config.New(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("error while loading config file: %s", cfgPath))
	}

	// create new db client
	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating db client")
	}

	btcClient, err := btcclient.NewBtcClient(&cfg.Btc)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc client")
	}

	qm, err := queue.NewQueueManager(&cfg.Queue)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating queue manager")
	}

	service := services.NewService(dbClient, btcClient, qm)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating delegation service")
	}

	// initialize metrics with the metrics port from config
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	service.StartIndexerSync(ctx)
}
