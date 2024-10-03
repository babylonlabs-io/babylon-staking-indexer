package queue

import (
	"github.com/rs/zerolog/log"

	queueConfig "github.com/babylonlabs-io/staking-queue-client/config"
)

type QueueManager struct {
}

func NewQueueManager(cfg *queueConfig.QueueConfig) (*QueueManager, error) {
	return &QueueManager{}, nil
}

// Shutdown gracefully stops the interaction with the queue, ensuring all resources are properly released.
func (qm *QueueManager) Shutdown() {
	log.Info().Msg("Shutting down queue manager")
}
