package services

import (
	"context"
	"fmt"
	"net/http"

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
	btcTip, err := s.btc.GetBlockCount()
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

		// Check if the delegation is in a qualified state to transition to Withdrawable
		if !utils.Contains(types.QualifiedStatesForExpired(), delegation.State) {
			log.Debug().
				Str("stakingTxHashHex", delegation.StakingTxHashHex).
				Str("currentState", delegation.State.String()).
				Msg("Ignoring expired delegation as it is not qualified to transition to Withdrawable")
			continue
		}

		if err := s.sendWithdrawableDelegationEvent(ctx, delegation); err != nil {
			return err
		}

		if err := s.db.UpdateBTCDelegationState(ctx, delegation.StakingTxHashHex, types.StateWithdrawable); err != nil {
			log.Error().Err(err).Msg("Error updating BTC delegation state to withdrawable")
			return types.NewInternalServiceError(
				fmt.Errorf("failed to update BTC delegation state to withdrawable: %w", err),
			)
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.StakingTxHashHex); err != nil {
			log.Error().Err(err).Msg("Error deleting expired delegation")
			return types.NewInternalServiceError(
				fmt.Errorf("failed to delete expired delegation: %w", err),
			)
		}
	}

	return nil
}
