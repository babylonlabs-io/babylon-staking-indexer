package services

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	ctypes "github.com/cometbft/cometbft/types"
	"github.com/rs/zerolog/log"
)

const (
	subscriberName = "babylon-staking-indexer"
	newBlockQuery  = "tm.event='NewBlock'"
)

func (s *Service) SubscribeToBbnEvents(ctx context.Context) {
	if !s.bbn.IsRunning() {
		log.Fatal().Msg("BBN client is not running")
	}

	eventChan, err := s.bbn.Subscribe(subscriberName, newBlockQuery)
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

				// Send the latest height to the BBN block processor
				s.latestHeightChan <- latestHeight

			case <-ctx.Done():
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
				Str("current_state", delegation.State.String()).
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
