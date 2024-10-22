package services

import (
	"context"
	"fmt"

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
	btcTip, err := s.btc.GetBlockCount()
	if err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to get BTC tip height: %w", err),
		)
	}

	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip))
	if err != nil {
		return types.NewInternalServiceError(
			fmt.Errorf("failed to find expired delegations: %w", err),
		)
	}

	for _, delegation := range expiredDelegations {
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
