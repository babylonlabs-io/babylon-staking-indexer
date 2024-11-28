package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	queueclient "github.com/babylonlabs-io/babylon-staking-indexer/internal/queue/client"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
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
	ev := queueclient.NewActiveStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)
	if err := s.queueManager.SendActiveStakingEvent(ctx, &ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to send active staking event: %w", err))
	}
	return nil
}

func (s *Service) sendUnbondingDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queueclient.NewUnbondingStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		delegation.StakingAmount,
	)
	if err := s.queueManager.SendUnbondingStakingEvent(ctx, &ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to send unbonding staking event: %w", err))
	}
	return nil
}
