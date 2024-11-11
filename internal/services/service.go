package services

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg               *config.Config
	db                db.DbInterface
	btc               btcclient.BtcInterface
	btcNotifier       notifier.ChainNotifier
	bbn               bbnclient.BbnInterface
	queueManager      *queue.QueueManager
	bbnEventProcessor chan BbnEvent
	latestHeightChan  chan int64
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btc btcclient.BtcInterface,
	btcNotifier notifier.ChainNotifier,
	bbn bbnclient.BbnInterface,
	qm *queue.QueueManager,
) *Service {
	eventProcessor := make(chan BbnEvent, eventProcessorSize)
	latestHeightChan := make(chan int64)
	return &Service{
		quit:              make(chan struct{}),
		cfg:               cfg,
		db:                db,
		btc:               btc,
		btcNotifier:       btcNotifier,
		bbn:               bbn,
		queueManager:      qm,
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

	// Sync global parameters
	s.SyncGlobalParams(ctx)
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
