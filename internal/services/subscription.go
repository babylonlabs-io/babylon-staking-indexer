package services

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	ctypes "github.com/cometbft/cometbft/types"
	"github.com/rs/zerolog/log"
)

const (
	subscriberName                  = "babylon-staking-indexer"
	newBlockQuery                   = "tm.event='NewBlock'"
	outCapacity                     = 100
	subscriptionHealthCheckInterval = 1 * time.Minute
	maxEventWaitInterval            = 1 * time.Minute
)

func (s *Service) SubscribeToBbnEvents(ctx context.Context) {
	if !s.bbn.IsRunning() {
		log.Fatal().Msg("BBN client is not running")
	}
	// Subscribe to new block events but only wait for 5 minutes for events
	// if nothing come through within 5 minutes, the underlying subscription will
	// be resubscribed.
	// This is a workaround for the fact that cometbft ws_client does not have
	// proper ping pong configuration setup to detect if the connection is dead.
	// Refer to https://github.com/cometbft/cometbft/commit/2fd8496bc109d010c6c2e415604131b500550e37#r151452099
	eventChan, err := s.bbn.Subscribe(
		subscriberName,
		newBlockQuery,
		subscriptionHealthCheckInterval,
		maxEventWaitInterval,
		outCapacity,
	)
	if err != nil {
		log.Fatal().Msgf("Failed to subscribe to events: %v", err)
	}

	go func() {
		for {
			select {
			case event := <-eventChan:
				newBlockEvent, ok := event.Data.(ctypes.EventDataNewBlock)
				if !ok {
					log.Fatal().Msg("Event is not a NewBlock event")
				}

				latestHeight := newBlockEvent.Block.Height
				if latestHeight == 0 {
					log.Fatal().Msg("Event doesn't contain block height information")
				}
				log.Debug().
					Int64("height", latestHeight).
					Msg("received new block event from babylon subscription")

				// Send the latest height to the BBN block processor
				s.latestHeightChan <- latestHeight

			case <-ctx.Done():
				log.Info().Msg("context done, unsubscribing all babylon events")
				err := s.bbn.UnsubscribeAll(subscriberName)
				if err != nil {
					log.Error().Msgf("Failed to unsubscribe from events: %v", err)
				}
				return
			}
		}
	}()
}

// Resubscribe to missed BTC notifications
func (s *Service) ResubscribeToMissedBtcNotifications(ctx context.Context) {
	go func() {
		log.Info().Msg("resubscribing to missed BTC notifications")
		delegations, err := s.db.GetBTCDelegationsByStates(ctx,
			[]types.DelegationState{
				types.StateActive,
				types.StateUnbonding,
				types.StateWithdrawable,
				types.StateSlashed,
			},
		)
		if err != nil {
			log.Fatal().Msgf("failed to get BTC delegations: %v", err)
		}

		for _, delegation := range delegations {
			if !delegation.HasInclusionProof() {
				log.Debug().
					Str("staking_tx", delegation.StakingTxHashHex).
					Str("reason", "missing_inclusion_proof").
					Msg("skip resubscribing to missed BTC notification")
				continue
			}

			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Stringer("current_state", delegation.State).
				Msg("resubscribing to missed BTC notification")

			// Register spend notification
			if err := s.registerStakingSpendNotification(
				ctx,
				delegation.StakingTxHashHex,
				delegation.StakingTxHex,
				delegation.StakingOutputIdx,
				delegation.StartHeight,
			); err != nil {
				log.Fatal().Msgf("failed to register spend notification: %v", err)
			}
		}
	}()
}
