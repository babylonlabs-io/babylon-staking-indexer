package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
)

func (s *Service) emitActiveDelegationEvent(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	stateHistory := make([]queuecli.StateRecord, len(delegation.StateHistory))
	for i, record := range delegation.StateHistory {
		stateHistory[i] = queuecli.StateRecord{
			State:    record.State.String(),    // Convert DelegationState to string
			SubState: record.SubState.String(), // Convert DelegationSubState to string
		}
	}

	stakingEvent := queuecli.NewActiveStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
		stateHistory,
	)

	if err := s.queueManager.PushActiveStakingEvent(&stakingEvent); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the staking event to the queue: %w", err))
	}
	return nil
}

func (s *Service) emitUnbondingDelegationEvent(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	stateHistory := make([]queuecli.StateRecord, len(delegation.StateHistory))
	for i, record := range delegation.StateHistory {
		stateHistory[i] = queuecli.StateRecord{
			State:    record.State.String(),    // Convert DelegationState to string
			SubState: record.SubState.String(), // Convert DelegationSubState to string
		}
	}

	ev := queuecli.NewUnbondingStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
		stateHistory,
	)
	if err := s.queueManager.PushUnbondingStakingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the unbonding event to the queue: %w", err))
	}
	return nil
}

func (s *Service) emitWithdrawableDelegationEvent(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	stateHistory := make([]queuecli.StateRecord, len(delegation.StateHistory))
	for i, record := range delegation.StateHistory {
		stateHistory[i] = queuecli.StateRecord{
			State:    record.State.String(),    // Convert DelegationState to string
			SubState: record.SubState.String(), // Convert DelegationSubState to string
		}
	}

	ev := queuecli.NewWithdrawableStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
		stateHistory,
	)
	if err := s.queueManager.PushWithdrawableStakingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the withdrawable event to the queue: %w", err))
	}
	return nil
}

func (s *Service) emitWithdrawnDelegationEvent(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	stateHistory := make([]queuecli.StateRecord, len(delegation.StateHistory))
	for i, record := range delegation.StateHistory {
		stateHistory[i] = queuecli.StateRecord{
			State:    record.State.String(),    // Convert DelegationState to string
			SubState: record.SubState.String(), // Convert DelegationSubState to string
		}
	}

	ev := queuecli.NewWithdrawnStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
		stateHistory,
	)
	if err := s.queueManager.PushWithdrawnStakingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the withdrawn event to the queue: %w", err))
	}
	return nil
}
