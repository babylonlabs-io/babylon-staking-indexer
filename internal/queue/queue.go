package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue/client"
	"github.com/rs/zerolog/log"

	queueConfig "github.com/babylonlabs-io/staking-queue-client/config"
)

type QueueManager struct {
	unbondingEventQueue     client.QueueClient
	activeStakingEventQueue client.QueueClient
}

func NewQueueManager(cfg *queueConfig.QueueConfig) (*QueueManager, error) {
	unbondingEventQueue, err := client.NewQueueClient(cfg, client.UnbondingStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize unbonding event queue: %w", err)
	}

	activeStakingEventQueue, err := client.NewQueueClient(cfg, client.ActiveStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize active staking event queue: %w", err)
	}

	return &QueueManager{
		unbondingEventQueue:     unbondingEventQueue,
		activeStakingEventQueue: activeStakingEventQueue,
	}, nil
}

func (qm *QueueManager) SendUnbondingStakingEvent(ctx context.Context, ev *client.StakingEvent) error {
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	messageBody := string(jsonBytes)

	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("pushing unbonding event")
	err = qm.unbondingEventQueue.SendMessage(ctx, messageBody)
	if err != nil {
		return fmt.Errorf("failed to push unbonding event: %w", err)
	}
	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("successfully pushed unbonding event")

	return nil
}

func (qm *QueueManager) SendActiveStakingEvent(ctx context.Context, ev *client.StakingEvent) error {
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	messageBody := string(jsonBytes)

	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("pushing active staking event")
	err = qm.activeStakingEventQueue.SendMessage(ctx, messageBody)
	if err != nil {
		return fmt.Errorf("failed to push active staking event: %w", err)
	}
	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("successfully pushed active staking event")

	return nil
}

// Shutdown gracefully stops the interaction with the queue, ensuring all resources are properly released.
func (qm *QueueManager) Shutdown() {
	err := qm.unbondingEventQueue.Stop()
	if err != nil {
		log.Error().Err(err).Msg("failed to stop unbonding event queue")
	}

	err = qm.activeStakingEventQueue.Stop()
	if err != nil {
		log.Error().Err(err).Msg("failed to stop active staking event queue")
	}
}
