package services

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue"
)

type Service struct {
	db                db.DbInterface
	btc               btcclient.BtcInterface
	bbn               bbnclient.BbnInterface
	queueManager      *queue.QueueManager
	bbnEventProcessor chan BbnEvent
}

func NewService(
	db db.DbInterface,
	btc btcclient.BtcInterface,
	bbn bbnclient.BbnInterface,
	qm *queue.QueueManager,
) *Service {
	eventProcessor := make(chan BbnEvent, eventProcessorSize)
	return &Service{
		db:                db,
		btc:               btc,
		bbn:               bbn,
		queueManager:      qm,
		bbnEventProcessor: eventProcessor,
	}
}

func (s *Service) StartIndexerSync(ctx context.Context) {
	// Start the bootstrap process
	s.bootstrapBbn(ctx)
	// Start the websocket event subscription process
	s.subscribeToBbnEvents(ctx)
	// Keep processing events in the main thread
	s.startBbnEventProcessor()
}
