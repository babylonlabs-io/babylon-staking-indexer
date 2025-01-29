package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
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
		if err := s.processExpiredDelegation(ctx, tlDoc); err != nil {
			return fmt.Errorf("failed to process expired delegation %s: %w", tlDoc.StakingTxHashHex, err)
		}
	}

	return nil
}

func (s *Service) processExpiredDelegation(ctx context.Context, tlDoc model.TimeLockDocument) error {
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, tlDoc.StakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegation: %w", err)
	}

	log.Debug().
		Str("staking_tx", delegation.StakingTxHashHex).
		Stringer("current_state", delegation.State).
		Stringer("new_sub_state", tlDoc.DelegationSubState).
		Uint32("expire_height", tlDoc.ExpireHeight).
		Msg("checking if delegation is expired")

	// If the delegation is already Withdrawn, the Withdrawable state is not needed
	// we should delete it from TimeLock collection so its not processed again
	if delegation.State == types.StateWithdrawn {
		return s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex)
	}

	// Determine the valid previous states that can transition to Withdrawable based on the delegation's sub-state
	var qualifiedStates []types.DelegationState
	switch tlDoc.DelegationSubState {
	case types.SubStateEarlyUnbonding, types.SubStateTimelock:
		// For normal unbonding flows (early unbonding or timelock expiry),
		// we expect the delegation to be in the Unbonding state.
		// State transition: Active -> Unbonding -> Withdrawable
		qualifiedStates = []types.DelegationState{types.StateUnbonding}

	case types.SubStateTimelockSlashing, types.SubStateEarlyUnbondingSlashing:
		// For slashing flows, we expect the delegation to be in the Slashed state.
		// This handles multiple scenarios:
		// 1. Slashing tx detected before Babylon events:
		//    Active -> Slashed -> Withdrawable
		// 2. Slashing tx detected after Babylon events:
		//    Active -> Unbonding -> Slashed -> Withdrawable
		// 3. User fails to withdraw within timelock window:
		//    Active -> Unbonding -> Withdrawable -> Slashed -> Withdrawable
		//    (SubState transitions from Timelock -> TimelockSlashing or
		//     EarlyUnbonding -> EarlyUnbondingSlashing)
		qualifiedStates = []types.DelegationState{types.StateSlashed}

	default:
		return fmt.Errorf("unknown delegation sub state: %s", tlDoc.DelegationSubState)
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

	return s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex)
}
