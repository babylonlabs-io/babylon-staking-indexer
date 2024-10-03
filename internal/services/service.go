package services

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/client/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue"
)

type Service struct {
	db           db.DbInterface
	btc          btcclient.BtcInterface
	queueManager *queue.QueueManager
}

func NewService(db db.DbInterface, btc btcclient.BtcInterface, qm *queue.QueueManager) *Service {
	return &Service{
		db:           db,
		btc:          btc,
		queueManager: qm,
	}
}

// Main entry point for the indexer service to start syncing with the blockchains
// This is a placeholder for now, we can move/restructure the location of the sync
// controller logic later
func (s *Service) StartIndexerSync(ctx context.Context) {
	// Step 1: Get the last processed BBN block number from the database
	// Step 2: Get the latest BBN block number from the blockchain
	// Step 3: Sync the blocks from the last processed block to the latest block
	// Step 3.1: Call `/block_result` endpoint to get the block results
	// Step 3.2: Decode the events data, feed into event processor channel (blocking)
	// Step 3.3: Update the last processed block number in the database
}
