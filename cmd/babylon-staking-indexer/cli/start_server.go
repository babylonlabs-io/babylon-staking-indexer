package cli

import (
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	dbmodel "github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/tracing"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
	"github.com/babylonlabs-io/staking-queue-client/queuemngr"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func StartServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-server",
		Short: "Starts the Babylon Staking Indexer server",
		Args:  cobra.ExactArgs(0),
		RunE:  startServer,
	}

	return cmd
}

func startServer(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	ctx = tracing.InjectTraceID(ctx)
	log := log.Ctx(ctx)

	// load config
	cfgPath := GetConfigPath()
	cfg, err := config.New(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("error while loading config file: %s", cfgPath))
	}

	err = dbmodel.Setup(ctx, &cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while setting up staking db model")
	}

	// create new db client
	var dbClient db.DbInterface
	dbClient, err = db.New(ctx, cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating db client")
	}
	dbClient = db.NewDbWithMetrics(dbClient)

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

	var btcClient btcclient.BtcInterface
	btcClient, err = btcclient.NewBTCClient(&cfg.BTC)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc client")
	}
	btcClient = btcclient.NewBTCClientWithMetrics(btcClient)

	bbnClient := bbnclient.NewBBNClient(&cfg.BBN)
	bbnClient = bbnclient.NewBBNClientWithMetrics(bbnClient)

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
	return nil
}
