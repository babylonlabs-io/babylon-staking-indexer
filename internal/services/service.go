package services

import (
	"context"

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
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btc btcclient.BtcInterface,
	bbn bbnclient.BbnInterface,
	qm *queue.QueueManager,
) *Service {
	eventProcessor := make(chan BbnEvent, eventProcessorSize)
	return &Service{
		cfg:               cfg,
		db:                db,
		btc:               btc,
		bbn:               bbn,
		queueManager:      qm,
		bbnEventProcessor: eventProcessor,
	}
}

func (s *Service) StartIndexerSync(ctx context.Context) {
	// Sync global parameters
	s.SyncGlobalParams(ctx)
	// Start the bootstrap process
	s.BootstrapBbn(ctx)
	// Start the websocket event subscription process
	s.SubscribeToBbnEvents(ctx)
	// Keep processing events in the main thread
	s.StartBbnEventProcessor(ctx)
}
