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
	stakingTxHashHex string,
	stakerBtcPkHex string,
	finalityProviderBtcPksHex []string,
	stakingAmount uint64,
) *types.Error {
	stakingEvent := queuecli.NewActiveStakingEvent(
		stakingTxHashHex,
		stakerBtcPkHex,
		finalityProviderBtcPksHex,
		stakingAmount,
	)

	if err := s.queueManager.PushActiveStakingEvent(&stakingEvent); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the staking event to the queue: %w", err))
	}
	return nil
}

func (s *Service) emitUnbondingDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queuecli.NewUnbondingStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)
	if err := s.queueManager.PushUnbondingStakingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the unbonding event to the queue: %w", err))
	}
	return nil
}

func (s *Service) emitWithdrawableDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queuecli.NewWithdrawableStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)
	if err := s.queueManager.PushWithdrawableStakingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the withdrawable event to the queue: %w", err))
	}
	return nil
}

func (s *Service) emitWithdrawnDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queuecli.NewWithdrawnStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)
	if err := s.queueManager.PushWithdrawnStakingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the withdrawn event to the queue: %w", err))
	}
	return nil
}
