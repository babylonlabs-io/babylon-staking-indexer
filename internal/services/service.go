package services

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/consumer"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg               *config.Config
	db                db.DbInterface
	btc               btcclient.BtcInterface
	btcNotifier       BtcNotifier
	bbn               bbnclient.BbnInterface
	queueManager      consumer.EventConsumer
	bbnEventProcessor chan BbnEvent
	latestHeightChan  chan int64
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btc btcclient.BtcInterface,
	btcNotifier BtcNotifier,
	bbn bbnclient.BbnInterface,
	consumer consumer.EventConsumer,
) *Service {
	eventProcessor := make(chan BbnEvent, eventProcessorSize)
	latestHeightChan := make(chan int64)
	// add retry wrapper to the btc notifier
	btcNotifier = newBtcNotifierWithRetries(btcNotifier)
	return &Service{
		quit:              make(chan struct{}),
		cfg:               cfg,
		db:                db,
		btc:               btc,
		btcNotifier:       btcNotifier,
		bbn:               bbn,
		queueManager:      consumer,
		bbnEventProcessor: eventProcessor,
		latestHeightChan:  latestHeightChan,
	}
}

func (s *Service) StartIndexerSync(ctx context.Context) {
	if err := s.bbn.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start BBN client")
	}

	if err := s.btcNotifier.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start btc chain notifier")
	}

	if err := s.queueManager.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start the event consumer")
	}

	// Sync global parameters
	s.SyncGlobalParams(ctx)
	// Resubscribe to missed BTC notifications
	s.ResubscribeToMissedBtcNotifications(ctx)
	// Start the expiry checker
	s.StartExpiryChecker(ctx)
	// Start the websocket event subscription process
	s.SubscribeToBbnEvents(ctx)
	// Keep processing BBN blocks in the main thread
	s.StartBbnBlockProcessor(ctx)
}

func (s *Service) quitContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	s.wg.Add(1)
	go func() {
		defer cancel()
		defer s.wg.Done()

		select {
		case <-s.quit:
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
