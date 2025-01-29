package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils/poller"
	"github.com/rs/zerolog/log"
)

func (s *Service) StartExpiryChecker(ctx context.Context) {
	expiryCheckerPoller := poller.NewPoller(
		s.cfg.Poller.ExpiryCheckerPollingInterval,
		s.checkExpiry,
	)
	go expiryCheckerPoller.Start(ctx)
}

func (s *Service) checkExpiry(ctx context.Context) error {
	btcTip, err := s.btc.GetTipHeight()
	if err != nil {
		return fmt.Errorf("failed to get BTC tip height: %w", err)
	}

	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, btcTip, s.cfg.Poller.ExpiredDelegationsLimit)
	if err != nil {
		return fmt.Errorf("failed to find expired delegations: %w", err)
	}

	for _, tlDoc := range expiredDelegations {
		delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, tlDoc.StakingTxHashHex)
		if err != nil {
			return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err)
		}

		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("current_state", delegation.State).
			Stringer("new_sub_state", tlDoc.DelegationSubState).
			Uint32("expire_height", tlDoc.ExpireHeight).
			Msg("checking if delegation is expired")

		qualifiedStates, err := types.QualifiedStatesForWithdrawable(tlDoc.DelegationSubState)
		if err != nil {
			return fmt.Errorf("failed to get qualified states for withdrawable: %w", err)
		}

		stateUpdateErr := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			qualifiedStates,
			types.StateWithdrawable,
			db.WithSubState(tlDoc.DelegationSubState),
			db.WithBtcHeight(tlDoc.ExpireHeight),
		)
		if stateUpdateErr != nil {
			if db.IsNotFoundError(stateUpdateErr) {
				log.Debug().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("skip updating BTC delegation state to withdrawable as the state is not qualified")
			} else {
				log.Error().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("failed to update BTC delegation state to withdrawable")
				return fmt.Errorf("failed to update BTC delegation state to withdrawable: %w", err)
			}
		} else {
			// This means the state transitioned to withdrawable so we need to emit the event
			if err := s.emitWithdrawableDelegationEvent(ctx, delegation); err != nil {
				return err
			}
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex); err != nil {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to delete expired delegation")
			return fmt.Errorf("failed to delete expired delegation: %w", err)
		}
	}

	return nil
}
