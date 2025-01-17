package main

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"go.uber.org/zap"

	"github.com/babylonlabs-io/babylon-staking-indexer/cmd/babylon-staking-indexer/cli"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	dbmodel "github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
	"github.com/babylonlabs-io/staking-queue-client/queuemngr"
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

	err = dbmodel.Setup(ctx, &cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while setting up staking db model")
	}

	// create new db client
	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating db client")
	}

	// Create a basic zap logger
	zapLogger, err := zap.NewProduction()
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating zap logger")
	}
	defer func() {
		if err := zapLogger.Sync(); err != nil {
			log.Fatal().Err(err).Msg("error while syncing zap logger")
		}
	}()

	queueConsumer, err := queuemngr.NewQueueManager(&cfg.Queue, zapLogger)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize event consumer")
	}

	btcClient, err := btcclient.NewBTCClient(&cfg.BTC)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc client")
	}

	bbnClient := bbnclient.NewBBNClient(&cfg.BBN)

	btcNotifier, err := btcclient.NewBTCNotifier(
		&cfg.BTC,
		&btcclient.EmptyHintCache{},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc notifier")
	}

	service := services.NewService(cfg, dbClient, btcClient, btcNotifier, bbnClient, queueConsumer)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating service")
	}

	// initialize metrics with the metrics port from config
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	service.StartIndexerSync(ctx)
}
