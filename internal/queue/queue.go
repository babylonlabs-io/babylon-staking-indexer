package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	queueConfig "github.com/babylonlabs-io/staking-queue-client/config"
)

type QueueManager struct {
	stakingExpiredEventQueue client.QueueClient
}

func NewQueueManager(cfg *queueConfig.QueueConfig) (*QueueManager, error) {
	stakingEventQueue, err := client.NewQueueClient(cfg, client.ExpiredStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize staking event queue: %w", err)
	}

	return &QueueManager{
		stakingExpiredEventQueue: stakingEventQueue,
	}, nil
}

func (qm *QueueManager) SendExpiredStakingEvent(ctx context.Context, ev client.ExpiredStakingEvent) error {
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	messageBody := string(jsonBytes)

	log.Debug().Str("tx_hash", ev.StakingTxHashHex).Msg("publishing expired staking event")
	err = qm.stakingExpiredEventQueue.SendMessage(ctx, messageBody)
	if err != nil {
		metrics.RecordQueueSendError()
		log.Fatal().Err(err).Str("tx_hash", ev.StakingTxHashHex).Msg("failed to publish staking event")
	}
	log.Debug().Str("tx_hash", ev.StakingTxHashHex).Msg("successfully published expired staking event")

	return nil
}

// Shutdown gracefully stops the interaction with the queue, ensuring all resources are properly released.
func (qm *QueueManager) Shutdown() {
	err := qm.stakingExpiredEventQueue.Stop()
	if err != nil {
		log.Error().Err(err).Msg("failed to stop staking expired event queue")
	}

}
