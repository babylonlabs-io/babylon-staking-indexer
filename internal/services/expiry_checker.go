package services

import (
	"context"
	"fmt"
	"net/http"
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

func (s *Service) checkExpiry(ctx context.Context) *types.Error {
	btcTip, err := s.btc.GetTipHeight()
	if err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get BTC tip height: %w", err),
		)
	}

	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, btcTip, s.cfg.Poller.ExpiredDelegationsLimit)
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
			Stringer("current_state", delegation.State).
			Stringer("new_sub_state", tlDoc.DelegationSubState).
			Uint32("expire_height", tlDoc.ExpireHeight).
			Msg("checking if delegation is expired")

		stateUpdateErr := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForWithdrawable(),
			types.StateWithdrawable,
			db.WithSubState(tlDoc.DelegationSubState),
			db.WithBtcHeight(int64(tlDoc.ExpireHeight)),
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
				return types.NewInternalServiceError(
					fmt.Errorf("failed to update BTC delegation state to withdrawable: %w", err),
				)
			}
		}

		if err := s.emitWithdrawableDelegationEvent(ctx, delegation); err != nil {
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
