package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
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

func (s *Service) checkExpiry(ctx context.Context) *types.Error {
	btcTip, err := s.btc.GetTipHeight()
	if err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get BTC tip height: %w", err),
		)
	}

	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip), s.cfg.Poller.ExpiredDelegationsLimit)
	if err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to find expired delegations: %w", err),
		)
	}

	for _, tlDoc := range expiredDelegations {
		delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, tlDoc.StakingTxHashHex)
		if err != nil {
			return types.NewError(
				http.StatusInternalServerError,
				types.InternalServiceError,
				fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err),
			)
		}

		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("current_state", delegation.State.String()).
			Str("new_sub_state", tlDoc.DelegationSubState.String()).
			Str("expire_height", strconv.FormatUint(uint64(tlDoc.ExpireHeight), 10)).
			Msg("checking if delegation is withdrawable")

		if utils.Contains(types.OutdatedStatesForWithdrawable(), delegation.State) {
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("current_state", delegation.State.String()).
				Msg("current state is outdated for withdrawable")

			if err := s.emitWithdrawableDelegationEvent(ctx, delegation); err != nil {
				log.Error().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("failed to emit withdrawable delegation event")
				return err
			}

			if err := s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex); err != nil {
				log.Error().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("failed to delete expired delegation")
				return types.NewInternalServiceError(
					fmt.Errorf("failed to delete expired delegation: %w", err),
				)
			}

			continue
		}

		// Check if the delegation is in a qualified state to transition to Withdrawable
		if !utils.Contains(types.QualifiedStatesForWithdrawable(), delegation.State) {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("current_state", delegation.State.String()).
				Msg("current state is not qualified for withdrawable")

			return types.NewInternalServiceError(
				fmt.Errorf("current state is not qualified for withdrawable"),
			)
		}

		if err := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForWithdrawable(),
			types.StateWithdrawable,
			&tlDoc.DelegationSubState,
		); err != nil {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to update BTC delegation state to withdrawable")
			return types.NewInternalServiceError(
				fmt.Errorf("failed to update BTC delegation state to withdrawable: %w", err),
			)
		}

		if err := s.emitWithdrawableDelegationEvent(ctx, delegation); err != nil {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to emit withdrawable delegation event")
			return err
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex); err != nil {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to delete expired delegation")
			return types.NewInternalServiceError(
				fmt.Errorf("failed to delete expired delegation: %w", err),
			)
		}
	}

	return nil
}
