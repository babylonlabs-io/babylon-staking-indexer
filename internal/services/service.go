package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/consumer"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
)

type Service struct {
	quit chan struct{}

	cfg                        *config.Config
	db                         db.DbInterface
	btc                        btcclient.BtcInterface
	btcNotifier                BtcNotifier
	bbn                        bbnclient.BbnInterface
	queueManager               consumer.EventConsumer
	bbnEventProcessor          chan BbnEvent
	latestHeightChan           chan int64
	stakingParamsLatestVersion uint32
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
		quit:                       make(chan struct{}),
		cfg:                        cfg,
		db:                         db,
		btc:                        btc,
		btcNotifier:                btcNotifier,
		bbn:                        bbn,
		queueManager:               consumer,
		bbnEventProcessor:          eventProcessor,
		latestHeightChan:           latestHeightChan,
		stakingParamsLatestVersion: 0,
	}
}

func (s *Service) StartIndexerSync(ctx context.Context) error {
	if err := s.bbn.Start(); err != nil {
		return fmt.Errorf("failed to start BBN client: %w", err)
	}

	if err := s.btcNotifier.Start(); err != nil {
		return fmt.Errorf("failed to start btc chain notifier: %w", err)
	}

	if err := s.queueManager.Start(); err != nil {
		return fmt.Errorf("failed to start the event consumer: %w", err)
	}

	// Sync global parameters
	s.SyncGlobalParams(ctx)
	// Resubscribe to missed BTC notifications
	s.ResubscribeToMissedBtcNotifications(ctx)
	// Start the expiry checker
	s.StartExpiryChecker(ctx)
	// Start the websocket event subscription process
	if err := s.SubscribeToBbnEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to BBN events: %w", err)
	}

	// Keep processing BBN blocks in the main thread
	return s.StartBbnBlockProcessor(ctx)
}

func (s *Service) quitContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		select {
		case <-s.quit:
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
