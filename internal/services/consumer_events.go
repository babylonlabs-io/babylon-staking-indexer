package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
)

func (s *Service) emitActiveDelegationEvent(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) error {
	stateHistoryStrs := model.ToStateStrings(delegation.StateHistory)
	stakingEvent := queuecli.NewActiveStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
		stateHistoryStrs,
	)

	if err := s.queueManager.PushActiveStakingEvent(ctx, &stakingEvent); err != nil {
		return fmt.Errorf("failed to push the staking event to the queue: %w", err)
	}
	return nil
}

// EmitBatchDelegationEvents sends events for multiple delegations concurrently
func (s *Service) EmitBatchDelegationEvents(
	ctx context.Context,
	delegations []*model.BTCDelegationDetails,
) []error {
	errors := make([]error, len(delegations))

	for i, delegation := range delegations {
		go func() {
			err := s.emitActiveDelegationEvent(ctx, delegation)
			errors[i] = err
		}()
	}

	// Give goroutines time to complete
	return errors
}

func (s *Service) emitUnbondingDelegationEvent(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) error {
	stateHistoryStrs := model.ToStateStrings(delegation.StateHistory)
	ev := queuecli.NewUnbondingStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
		stateHistoryStrs,
	)
	if err := s.queueManager.PushUnbondingStakingEvent(ctx, &ev); err != nil {
		return fmt.Errorf("failed to push the unbonding event to the queue: %w", err)
	}
	return nil
}
