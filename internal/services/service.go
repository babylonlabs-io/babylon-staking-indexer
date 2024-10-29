package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue"
)

type Service struct {
	cfg               *config.Config
	db                db.DbInterface
	btc               btcclient.BtcInterface
	bbn               bbnclient.BbnInterface
	queueManager      *queue.QueueManager
	bbnEventProcessor chan BbnEvent
	latestHeightChan  chan int64
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btc btcclient.BtcInterface,
	bbn bbnclient.BbnInterface,
	qm *queue.QueueManager,
) *Service {
	eventProcessor := make(chan BbnEvent, eventProcessorSize)
	latestHeightChan := make(chan int64)

	if err := bbn.Start(); err != nil {
		panic(fmt.Errorf("failed to start BBN client: %w", err))
	}

	return &Service{
		cfg:               cfg,
		db:                db,
		btc:               btc,
		bbn:               bbn,
		queueManager:      qm,
		bbnEventProcessor: eventProcessor,
		latestHeightChan:  latestHeightChan,
	}
}

func (s *Service) StartIndexerSync(ctx context.Context) {
	// Sync global parameters
	s.SyncGlobalParams(ctx)
	// Start the expiry checker
	s.StartExpiryChecker(ctx)
	// Start the bootstrap process
	s.BootstrapBbn(ctx)
	// Start the websocket event subscription process
	s.SubscribeToBbnEvents(ctx)
	// Keep processing events in the main thread
	s.StartBbnEventProcessor(ctx)
}
