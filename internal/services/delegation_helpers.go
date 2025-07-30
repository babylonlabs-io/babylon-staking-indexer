package services

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
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
		spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
			&unbondingOutpoint,
			unbondingTx.TxOut[0].PkScript,
			delegation.StartHeight,
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

// evaluateCanExpand determines if a delegation can be expanded based on business logic:
// 1. Delegation must be active
// 2. At least one of these conditions must be met:
//    a. Tx-hash appears in the allow-list, OR
//    b. The delegation contains > 1 finality providers
func (s *Service) evaluateCanExpand(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
) (bool, error) {
	// Condition 1: Delegation must be active
	if delegation.State != types.StateActive {
		return false, nil
	}

	// TODO: Add allow-list expiration check using AllowListExpirationHeight from network_info
	// This should check if the allow-list is still active based on current block height

	// Condition 2a: Check if tx-hash appears in allow-list
	// Check if this delegation was previously marked as can_expand via CLI import
	// This indicates it was in the allow-list
	if delegation.CanExpand {
		return true, nil
	}

	// Condition 2b: Check if delegation contains > 1 finality providers
	if len(delegation.FinalityProviderBtcPksHex) > 1 {
		return true, nil
	}

	return false, nil
}

// updateDelegationCanExpand updates the can_expand field for a delegation
func (s *Service) updateDelegationCanExpand(
	ctx context.Context,
	stakingTxHash string,
) error {
	// Get the current delegation
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if err != nil {
		return fmt.Errorf("failed to get delegation: %w", err)
	}

	// Evaluate can_expand
	canExpand, err := s.evaluateCanExpand(ctx, delegation)
	if err != nil {
		return fmt.Errorf("failed to evaluate can_expand: %w", err)
	}

	// Update if the value has changed
	if delegation.CanExpand != canExpand {
		if canExpand {
			if err := s.db.SetBTCDelegationCanExpand(ctx, stakingTxHash); err != nil {
				return fmt.Errorf("failed to set can_expand to true: %w", err)
			}
		} else {
			// Note: We don't have a method to set can_expand to false in the current DB interface
			// This is because the business logic typically only moves from false to true
			// If needed, we can add this method later
			log.Debug().
				Str("staking_tx", stakingTxHash).
				Msg("delegation no longer qualifies for expansion but not updating to false")
		}
	}

	return nil
}
