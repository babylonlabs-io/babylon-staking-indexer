package services

import (
	"context"
	"fmt"
	"slices"

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

		// Handle already withdrawn delegations
		if delegation.State == types.StateWithdrawn {
			if err := s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex); err != nil {
				return fmt.Errorf("failed to delete expired delegation: %w", err)
			}
			continue
		}

		qualifiedStates, err := types.QualifiedStatesForWithdrawable(tlDoc.DelegationSubState)
		if err != nil {
			return fmt.Errorf("failed to get qualified states: %w", err)
		}

		// Skip if current state is not qualified for transition
		if !slices.Contains(qualifiedStates, delegation.State) {
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Stringer("current_state", delegation.State).
				Msg("skipping expired delegation, current state not qualified for transition")
			continue
		}

		// Update delegation state
		if err := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			qualifiedStates,
			types.StateWithdrawable,
			db.WithSubState(tlDoc.DelegationSubState),
			db.WithBtcHeight(int64(tlDoc.ExpireHeight)),
		); err != nil {
			return fmt.Errorf("failed to update delegation state: %w", err)
		}

		// Emit event and cleanup
		if err := s.emitWithdrawableDelegationEvent(ctx, delegation); err != nil {
			return fmt.Errorf("failed to emit withdrawable event: %w", err)
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex); err != nil {
			return fmt.Errorf("failed to delete expired delegation: %w", err)
		}
	}

	return nil
}
