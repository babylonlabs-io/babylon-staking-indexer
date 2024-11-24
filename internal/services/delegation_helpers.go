package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func (s *Service) registerUnbondingSpendNotification(
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

func (s *Service) registerStakingSpendNotification(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) *types.Error {
	stakingTxHash, err := chainhash.NewHashFromStr(delegation.StakingTxHashHex)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking tx hash: %w", err),
		)
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to deserialize staking tx: %w", err),
		)
	}

	stakingOutpoint := wire.OutPoint{
		Hash:  *stakingTxHash,
		Index: delegation.StakingOutputIdx,
	}

	spendEv, err := s.btcNotifier.RegisterSpendNtfn(
		&stakingOutpoint,
		stakingTx.TxOut[delegation.StakingOutputIdx].PkScript,
		delegation.StartHeight,
	)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to register spend ntfn for staking tx %s: %w", delegation.StakingTxHashHex, err),
		)
	}

	s.wg.Add(1)
	go s.watchForSpendStakingTx(spendEv, delegation)

	return nil
}
