package services

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/rs/zerolog/log"
)

func (s *Service) registerUnbondingSpendNotification(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) error {
	unbondingTxBytes, parseErr := hex.DecodeString(delegation.UnbondingTx)
	if parseErr != nil {
		return fmt.Errorf("failed to decode unbonding tx: %w", parseErr)
	}

	unbondingTx, parseErr := bbn.NewBTCTxFromBytes(unbondingTxBytes)
	if parseErr != nil {
		return fmt.Errorf("failed to parse unbonding tx: %w", parseErr)
	}

	log := log.Ctx(ctx)
	log.Debug().
		Str("staking_tx", delegation.StakingTxHashHex).
		Stringer("unbonding_tx", unbondingTx.TxHash()).
		Msg("registering early unbonding spend notification")

	unbondingOutpoint := wire.OutPoint{
		Hash:  unbondingTx.TxHash(),
		Index: 0, // unbonding tx has only 1 output
	}

	s.wg.Go(func() {
		spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
			&unbondingOutpoint,
			unbondingTx.TxOut[0].PkScript,
			delegation.StartHeight,
		)
		metrics.IncBtcNotifierRegisterSpend(btcErr != nil)
		if btcErr != nil {
			log.Error().Err(btcErr).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to register unbonding spend notification")
			return
		}

		s.watchForSpendUnbondingTx(ctx, spendEv, delegation)
	})

	return nil
}

func (s *Service) registerStakingSpendNotification(
	ctx context.Context,
	stakingTxHashHex string,
	stakingTxHex string,
	stakingOutputIdx uint32,
	stakingStartHeight uint32,
) error {
	stakingTxHash, err := chainhash.NewHashFromStr(stakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to parse staking tx hash: %w", err)
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(stakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingOutpoint := wire.OutPoint{
		Hash:  *stakingTxHash,
		Index: stakingOutputIdx,
	}

	s.wg.Go(func() {
		spendEv, err := s.btcNotifier.RegisterSpendNtfn(
			&stakingOutpoint,
			stakingTx.TxOut[stakingOutputIdx].PkScript,
			stakingStartHeight,
		)
		metrics.IncBtcNotifierRegisterSpend(err != nil)
		if err != nil {
			log.Error().Err(err).
				Str("staking_tx", stakingTxHashHex).
				Msg("failed to register staking spend notification")
			return
		}

		s.watchForSpendStakingTx(ctx, spendEv, stakingTxHashHex)
	})

	return nil
}
