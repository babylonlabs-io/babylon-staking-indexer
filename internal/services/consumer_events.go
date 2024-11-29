package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
)

func (s *Service) emitConsumerEvent(
	ctx context.Context, newState types.DelegationState, delegation *model.BTCDelegationDetails,
) *types.Error {
	switch newState {
	case types.StateActive:
		return s.sendActiveDelegationEvent(ctx, delegation)
	case types.StateUnbonding:
		return s.sendUnbondingDelegationEvent(ctx, delegation)
	default:
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("unknown delegation state: %s", newState),
		)
	}
}

// TODO: fix the queue event schema
func (s *Service) sendActiveDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	stakingEvent := queuecli.NewActiveStakingEventV2(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)

	if err := s.queueManager.PushStakingEvent(&stakingEvent); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the staking event to the queue: %w", err))
	}
	return nil
}

func (s *Service) sendUnbondingDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queuecli.NewUnbondingStakingEventV2(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)
	if err := s.queueManager.PushUnbondingEvent(&ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to push the unbonding event to the queue: %w", err))
	}
	return nil
}
