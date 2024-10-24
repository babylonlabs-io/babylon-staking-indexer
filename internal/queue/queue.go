package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/queue/client"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	queueConfig "github.com/babylonlabs-io/staking-queue-client/config"
)

type QueueManager struct {
	stakingExpiredEventQueue  client.QueueClient
	unbondingEventQueue       client.QueueClient
	activeStakingEventQueue   client.QueueClient
	verifiedStakingEventQueue client.QueueClient
	pendingStakingEventQueue  client.QueueClient
}

func NewQueueManager(cfg *queueConfig.QueueConfig) (*QueueManager, error) {
	stakingEventQueue, err := client.NewQueueClient(cfg, client.ExpiredStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize staking event queue: %w", err)
	}

	unbondingEventQueue, err := client.NewQueueClient(cfg, client.UnbondingStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize unbonding event queue: %w", err)
	}

	activeStakingEventQueue, err := client.NewQueueClient(cfg, client.ActiveStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize active staking event queue: %w", err)
	}

	verifiedStakingEventQueue, err := client.NewQueueClient(cfg, client.VerifiedStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verified staking event queue: %w", err)
	}

	pendingStakingEventQueue, err := client.NewQueueClient(cfg, client.PendingStakingQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pending staking event queue: %w", err)
	}

	return &QueueManager{
		stakingExpiredEventQueue:  stakingEventQueue,
		unbondingEventQueue:       unbondingEventQueue,
		activeStakingEventQueue:   activeStakingEventQueue,
		verifiedStakingEventQueue: verifiedStakingEventQueue,
		pendingStakingEventQueue:  pendingStakingEventQueue,
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

func (qm *QueueManager) SendUnbondingEvent(ctx context.Context, ev *client.UnbondingStakingEvent) error {
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	messageBody := string(jsonBytes)

	log.Info().Str("staking_tx_hash", ev.UnbondingTxHashHex).Msg("pushing unbonding event")
	err = qm.unbondingEventQueue.SendMessage(ctx, messageBody)
	if err != nil {
		return fmt.Errorf("failed to push unbonding event: %w", err)
	}
	log.Info().Str("staking_tx_hash", ev.UnbondingTxHashHex).Msg("successfully pushed unbonding event")

	return nil
}

func (qm *QueueManager) SendActiveStakingEvent(ctx context.Context, ev *client.ActiveStakingEvent) error {
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

func (qm *QueueManager) SendVerifiedStakingEvent(ctx context.Context, ev *client.VerifiedStakingEvent) error {
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	messageBody := string(jsonBytes)

	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("pushing verified staking event")
	err = qm.verifiedStakingEventQueue.SendMessage(ctx, messageBody)
	if err != nil {
		return fmt.Errorf("failed to push verified staking event: %w", err)
	}
	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("successfully pushed verified staking event")

	return nil
}

func (qm *QueueManager) SendPendingStakingEvent(ctx context.Context, ev *client.PendingStakingEvent) error {
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	messageBody := string(jsonBytes)

	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("pushing pending staking event")
	err = qm.pendingStakingEventQueue.SendMessage(ctx, messageBody)
	if err != nil {
		return fmt.Errorf("failed to push pending staking event: %w", err)
	}
	log.Info().Str("staking_tx_hash", ev.StakingTxHashHex).Msg("successfully pushed pending staking event")

	return nil
}

// Shutdown gracefully stops the interaction with the queue, ensuring all resources are properly released.
func (qm *QueueManager) Shutdown() {
	err := qm.stakingExpiredEventQueue.Stop()
	if err != nil {
		log.Error().Err(err).Msg("failed to stop staking expired event queue")
	}

}
