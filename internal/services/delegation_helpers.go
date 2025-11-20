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

	go func() {
		// Use unbonding start height as hint instead of delegation start height
		// to help LND narrow down the rescan range for the unbonding tx output
		heightHint := delegation.StartHeight
		if delegation.UnbondingStartHeight > 0 {
			heightHint = delegation.UnbondingStartHeight
		}

		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("unbonding_tx", unbondingTx.TxHash()).
			Uint32("height_hint", heightHint).
			Uint32("delegation_start_height", delegation.StartHeight).
			Uint32("unbonding_start_height", delegation.UnbondingStartHeight).
			Msg("calling RegisterSpendNtfn for unbonding tx")

		spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
			&unbondingOutpoint,
			unbondingTx.TxOut[0].PkScript,
			heightHint,
		)
		metrics.IncBtcNotifierRegisterSpend(btcErr != nil)
		if btcErr != nil {
			// TODO: Handle the error in a better way such as retrying immediately
			// If continue to fail, we could retry by sending to queue and processing
			// later again to make sure we don't miss any spend
			// Will leave it as it is for now with alerts on log
			log.Error().Err(btcErr).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to register unbonding spend notification")
			return
		}

		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("unbonding_tx", unbondingTx.TxHash()).
			Msg("RegisterSpendNtfn succeeded, now waiting for spend event")

		s.watchForSpendUnbondingTx(ctx, spendEv, delegation)
	}()

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

	go func() {
		spendEv, err := s.btcNotifier.RegisterSpendNtfn(
			&stakingOutpoint,
			stakingTx.TxOut[stakingOutputIdx].PkScript,
			stakingStartHeight,
		)
		metrics.IncBtcNotifierRegisterSpend(err != nil)
		if err != nil {
			// TODO: Handle the error in a better way such as retrying immediately
			// If continue to fail, we could retry by sending to queue and processing
			// later again to make sure we don't miss any spend
			// Will leave it as it is for now with alerts on log
			log.Error().Err(err).
				Str("staking_tx", stakingTxHashHex).
				Msg("failed to register staking spend notification")
			return
		}

		s.watchForSpendStakingTx(ctx, spendEv, stakingTxHashHex)
	}()

	return nil
}
