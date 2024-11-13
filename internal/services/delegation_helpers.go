package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbn "github.com/babylonlabs-io/babylon/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/wire"
)

// Delegation helper functions
func (s *Service) getDelegationDetails(
	ctx context.Context,
	stakingTxHash string,
) (*model.BTCDelegationDetails, *types.Error) {
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if dbErr != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}
	return delegation, nil
}

func (s *Service) handleUnbondingProcess(
	ctx context.Context,
	event *bbntypes.EventBTCDelgationUnbondedEarly,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	unbondingStartHeight, parseErr := strconv.ParseUint(event.StartHeight, 10, 32)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse start height: %w", parseErr),
		)
	}

	// Save timelock expire
	unbondingExpireHeight := uint32(unbondingStartHeight) + delegation.UnbondingTime
	if err := s.db.SaveNewTimeLockExpire(
		ctx,
		delegation.StakingTxHashHex,
		unbondingExpireHeight,
		types.EarlyUnbondingTxType.String(),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save timelock expire: %w", err),
		)
	}

	// Update delegation state
	if err := s.db.UpdateBTCDelegationState(
		ctx,
		event.StakingTxHash,
		types.StateUnbonding,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

func (s *Service) startWatchingUnbondingSpend(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	unbondingTxBytes, parseErr := hex.DecodeString(delegation.UnbondingTx)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to decode unbonding tx: %w", parseErr),
		)
	}

	unbondingTx, parseErr := bbn.NewBTCTxFromBytes(unbondingTxBytes)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse unbonding tx: %w", parseErr),
		)
	}

	unbondingOutpoint := wire.OutPoint{
		Hash:  unbondingTx.TxHash(),
		Index: 0, // unbonding tx has only 1 output
	}

	spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
		&unbondingOutpoint,
		unbondingTx.TxOut[0].PkScript,
		delegation.StartHeight,
	)
	if btcErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to register spend ntfn for unbonding tx %s: %w", delegation.StakingTxHashHex, btcErr),
		)
	}

	s.wg.Add(1)
	go s.watchForSpendUnbondingTx(spendEv, delegation)

	return nil
}
