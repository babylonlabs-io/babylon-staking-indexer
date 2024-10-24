package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	queueclient "github.com/babylonlabs-io/babylon-staking-indexer/internal/queue/client"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) emitConsumerEvent(
	ctx context.Context, newState types.DelegationState, delegation *model.BTCDelegationDetails,
) *types.Error {
	switch newState {
	case types.StateActive:
		return s.sendActiveDelegationEvent(ctx, delegation)
	case types.StateVerified:
		return s.sendVerifiedDelegationEvent(ctx, delegation)
	case types.StatePending:
		return s.sendPendingDelegationEvent(ctx, delegation)
	case types.StateUnbonding:
		return s.sendUnbondingDelegationEvent(ctx, delegation)
	case types.StateWithdrawable:
		return s.sendWithdrawableDelegationEvent(ctx, delegation)
	default:
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("unknown delegation state: %s", newState),
		)
	}
}

func (s *Service) sendActiveDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	stakingTime, _ := strconv.ParseUint(delegation.StakingTime, 10, 64)
	stakingAmount, _ := strconv.ParseUint(delegation.StakingAmount, 10, 64)
	ev := queueclient.NewActiveStakingEvent(
		delegation.StakingTxHashHex,
		delegation.StakerBtcPkHex,
		delegation.FinalityProviderBtcPksHex,
		stakingAmount,
		uint64(delegation.StartHeight),
		time.Now().Unix(),
		stakingTime,
		0,
		"",
		false,
	)
	if err := s.queueManager.SendActiveStakingEvent(ctx, &ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to send active staking event: %w", err))
	}
	return nil
}

func (s *Service) sendVerifiedDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queueclient.NewVerifiedStakingEvent(delegation.StakingTxHashHex)
	if err := s.queueManager.SendVerifiedStakingEvent(ctx, &ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to send verified staking event: %w", err))
	}
	return nil
}

func (s *Service) sendPendingDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queueclient.NewPendingStakingEvent(delegation.StakingTxHashHex)
	if err := s.queueManager.SendPendingStakingEvent(ctx, &ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to send pending staking event: %w", err))
	}
	return nil
}

func (s *Service) sendUnbondingDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queueclient.NewUnbondingStakingEvent(
		delegation.StakingTxHashHex,
		uint64(delegation.EndHeight),
		time.Now().Unix(),
		uint64(delegation.StartHeight),
		uint64(delegation.EndHeight),
		delegation.UnbondingTx,
		delegation.UnbondingTime,
	)
	if err := s.queueManager.SendUnbondingStakingEvent(ctx, &ev); err != nil {
		return types.NewInternalServiceError(fmt.Errorf("failed to send unbonding staking event: %w", err))
	}
	return nil
}

func (s *Service) sendWithdrawableDelegationEvent(ctx context.Context, delegation *model.BTCDelegationDetails) *types.Error {
	ev := queueclient.NewExpiredStakingEvent(delegation.StakingTxHashHex, "") // TODO: add the correct tx type
	if err := s.queueManager.SendExpiredStakingEvent(ctx, ev); err != nil {
		log.Error().Err(err).Msg("Error sending expired staking event")
		return types.NewInternalServiceError(
			fmt.Errorf("failed to send expired staking event: %w", err),
		)
	}

	return nil
}
