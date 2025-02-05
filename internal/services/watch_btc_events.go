package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"

	"slices"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/rs/zerolog/log"
)

func (s *Service) watchForSpendStakingTx(
	spendEvent *notifier.SpendEvent,
	stakingTxHashHex string,
) {
	quitCtx, cancel := s.quitContext()
	defer cancel()

	// Get spending details
	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", stakingTxHashHex).
			Stringer("spending_tx", spendDetail.SpendingTx.TxHash()).
			Msg("staking tx has been spent")
		if err := s.handleSpendingStakingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			uint32(spendDetail.SpendingHeight),
			stakingTxHashHex,
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", stakingTxHashHex).
				Stringer("spending_tx", spendDetail.SpendingTx.TxHash()).
				Msg("failed to handle spending staking transaction")
			return
		}

	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}

}

func (s *Service) watchForSpendUnbondingTx(
	spendEvent *notifier.SpendEvent,
	delegation *model.BTCDelegationDetails,
) {
	defer s.wg.Done()
	quitCtx, cancel := s.quitContext()
	defer cancel()

	// Get spending details
	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("unbonding_tx", spendDetail.SpendingTx.TxHash()).
			Msg("unbonding tx has been spent")
		if err := s.handleSpendingUnbondingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			uint32(spendDetail.SpendingHeight),
			spendDetail.SpenderInputIndex,
			delegation,
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Stringer("unbonding_tx", spendDetail.SpendingTx.TxHash()).
				Msg("failed to handle spending unbonding transaction")
			return
		}

	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}
}

func (s *Service) watchForSpendSlashingChange(
	spendEvent *notifier.SpendEvent,
	delegation *model.BTCDelegationDetails,
	subState types.DelegationSubState,
) {
	defer s.wg.Done()
	quitCtx, cancel := s.quitContext()
	defer cancel()

	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("spending_tx", spendDetail.SpendingTx.TxHash()).
			Msg("slashing change output has been spent")
		delegationState, err := s.db.GetBTCDelegationState(quitCtx, delegation.StakingTxHashHex)
		if err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to get delegation state")
			return
		}

		qualifiedStates := types.QualifiedStatesForWithdrawn()
		if qualifiedStates == nil || !slices.Contains(qualifiedStates, *delegationState) {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Stringer("state", delegationState).
				Msg("current state is not qualified for slashed withdrawn")
			return
		}

		if err := s.emitWithdrawnDelegationEvent(quitCtx, delegation); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to emit withdrawn delegation event")
			return
		}

		// Update to withdrawn state
		delegationSubState := subState
		if err := s.db.UpdateBTCDelegationState(
			quitCtx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForWithdrawn(),
			types.StateWithdrawn,
			db.WithSubState(delegationSubState),
			db.WithBtcHeight(uint32(spendDetail.SpendingHeight)),
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Stringer("state", types.StateWithdrawn).
				Stringer("sub_state", delegationSubState).
				Msg("failed to update delegation state to withdrawn")
			return
		}

	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}
}

func (s *Service) handleSpendingStakingTransaction(
	ctx context.Context,
	spendingTx *wire.MsgTx,
	spendingInputIdx uint32,
	spendingHeight uint32,
	stakingTxHashHex string,
) error {
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err)
	}

	params, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	// Try to validate as unbonding transaction
	isUnbonding, err := s.isSpendingStakingTxUnbondingPath(spendingTx, delegation, params)
	if err != nil {
		return fmt.Errorf("failed to check staking tx unbonding path: %w", err)
	}
	if isUnbonding {
		// early unbonding has been detected, this could be
		// valid unbonding tx or unexpected unbonding tx
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("unbonding_tx", spendingTx.TxHash()).
			Msg("staking tx has been spent through unbonding path")

		unbondingBtcTimestamp, err := s.btc.GetBlockTimestamp(spendingHeight)
		if err != nil {
			return fmt.Errorf("failed to get block timestamp: %w", err)
		}

		// update delegation state to unbonding/early unbonding
		subState := types.SubStateEarlyUnbonding
		if err := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForUnbondedEarly(),
			types.StateUnbonding,
			db.WithSubState(subState),
			db.WithBtcHeight(spendingHeight),
			db.WithUnbondingBTCTimestamp(unbondingBtcTimestamp),
			db.WithUnbondingStartHeight(spendingHeight),
		); err != nil {
			if db.IsNotFoundError(err) {
				// maybe the babylon event processBTCDelegationUnbondedEarlyEvent is already
				// processed and updated the state
				log.Debug().
					Str("staking_tx", delegation.StakingTxHashHex).
					Interface("qualified_states", types.QualifiedStatesForUnbondedEarly()).
					Msg("delegation not in qualified states for early unbonding update")
				return nil
			}
			return fmt.Errorf("failed to update BTC delegation state: %w", err)
		}

		// check if the unbonding tx output is valid
		// this is important to identify if the spending tx is a valid unbonding tx
		validUnbondingOutput, err := s.validateUnbondingTxOutput(spendingTx, delegation, params)
		if err != nil {
			return fmt.Errorf("failed to validate unbonding tx output: %w", err)
		}
		if !validUnbondingOutput {
			// no action is needed if unexpected unbonding tx is detected
			// we should not subscribe to the unbonding tx spend notification
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("spending_tx", spendingTx.TxHash().String()).
				Msg("detected unexpected unbonding transaction")
			return nil
		}
		// the unbonding output is valid and matches the registered unbonding tx in babylon
		// emit consumer event to notify API
		if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
			return err
		}

		// Save timelock expire
		unbondingExpireHeight := uint32(spendingHeight) + delegation.UnbondingTime
		if err := s.db.SaveNewTimeLockExpire(
			ctx,
			delegation.StakingTxHashHex,
			unbondingExpireHeight,
			subState,
		); err != nil {
			return fmt.Errorf("failed to save timelock expire: %w", err)
		}

		// register unbonding spend notification
		return s.registerUnbondingSpendNotification(ctx, delegation)

	}

	// Try to validate as withdrawal transaction
	isWithdrawal, err := s.isSpendingStakingTxTimeLockPath(spendingTx, spendingInputIdx, delegation, params)
	if err != nil {
		return fmt.Errorf("failed to validate withdrawal tx: %w", err)
	}
	if isWithdrawal {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("withdrawal_tx", spendingTx.TxHash()).
			Msg("staking tx has been spent through withdrawal path")
		return s.handleWithdrawal(ctx, delegation, types.SubStateTimelock, spendingHeight)
	}

	// Try to validate as slashing transaction
	isSlashing, err := s.isSpendingStakingTxSlashingPath(spendingTx, spendingInputIdx, delegation, params)
	if err != nil {
		return fmt.Errorf("failed to validate slashing tx: %w", err)
	}
	if isSlashing {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("slashing_tx", spendingTx.TxHash().String()).
			Msg("staking tx has been spent through slashing path")

		// Save slashing tx hex
		slashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(spendingTx)
		if err != nil {
			return fmt.Errorf("failed to convert slashing tx to bytes: %w", err)
		}
		slashingTxHex := slashingTx.ToHexStr()

		// TODO: emit slashing event in a dedicated queue
		// refer https://github.com/babylonlabs-io/babylon-staking-indexer/issues/141
		if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
			return err
		}

		slashingBtcTimestamp, err := s.btc.GetBlockTimestamp(spendingHeight)
		if err != nil {
			return fmt.Errorf("failed to get block timestamp: %w", err)
		}

		// Update state and slashing related fields
		if err := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForSlashed(),
			types.StateSlashed,
			db.WithSubState(types.SubStateTimelockSlashing),
			db.WithStakingSlashingTx(slashingTxHex, spendingHeight, slashingBtcTimestamp),
			db.WithBtcHeight(spendingHeight),
		); err != nil {
			return fmt.Errorf("failed to update BTC delegation state: %w", err)
		}

		// It's a valid slashing tx, watch for spending change output
		return s.startWatchingSlashingChange(
			ctx,
			spendingTx,
			spendingHeight,
			delegation,
			types.SubStateTimelockSlashing,
		)
	}

	return fmt.Errorf("spending tx is neither unbonding nor withdrawal nor slashing")
}

func (s *Service) handleSpendingUnbondingTransaction(
	ctx context.Context,
	spendingTx *wire.MsgTx,
	spendingHeight uint32,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
) error {
	params, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	// First try to validate as withdrawal transaction
	isWithdrawal, err := s.isSpendingUnbondingTxTimeLockPath(spendingTx, delegation, spendingInputIdx, params)
	if err != nil {
		return fmt.Errorf("failed to validate withdrawal tx: %w", err)
	}
	if isWithdrawal {
		// It's a valid withdrawal, process it
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("unbonding_tx", spendingTx.TxHash()).
			Msg("unbonding tx has been spent through withdrawal path")
		return s.handleWithdrawal(ctx, delegation, types.SubStateEarlyUnbonding, spendingHeight)
	}

	// Try to validate as slashing transaction
	isSlashing, err := s.isSpendingUnbondingTxSlashingPath(spendingTx, delegation, spendingInputIdx, params)
	if err != nil {
		return fmt.Errorf("failed to validate slashing tx: %w", err)
	}
	if isSlashing {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("slashing_tx", spendingTx.TxHash().String()).
			Msg("unbonding tx has been spent through slashing path")

		// Save unbonding slashing tx hex
		unbondingSlashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(spendingTx)
		if err != nil {
			return fmt.Errorf("failed to convert unbonding slashing tx to bytes: %w", err)
		}
		unbondingSlashingTxHex := unbondingSlashingTx.ToHexStr()

		unbondingSlashingBtcTimestamp, err := s.btc.GetBlockTimestamp(spendingHeight)
		if err != nil {
			return fmt.Errorf("failed to get block timestamp: %w", err)
		}

		// Update state and slashing related fields
		if err := s.db.UpdateBTCDelegationState(
			ctx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForSlashed(),
			types.StateSlashed,
			db.WithSubState(types.SubStateEarlyUnbondingSlashing),
			db.WithUnbondingSlashingTx(unbondingSlashingTxHex, spendingHeight, unbondingSlashingBtcTimestamp),
			db.WithBtcHeight(spendingHeight),
		); err != nil {
			return fmt.Errorf("failed to update BTC delegation state: %w", err)
		}

		// It's a valid slashing tx, watch for spending change output
		return s.startWatchingSlashingChange(
			ctx,
			spendingTx,
			spendingHeight,
			delegation,
			types.SubStateEarlyUnbondingSlashing,
		)
	}

	return fmt.Errorf("spending tx is neither withdrawal nor slashing")
}

func (s *Service) handleWithdrawal(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
	subState types.DelegationSubState,
	spendingHeight uint32,
) error {
	delegationState, err := s.db.GetBTCDelegationState(ctx, delegation.StakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get delegation state: %w", err)
	}

	qualifiedStates := types.QualifiedStatesForWithdrawn()
	if qualifiedStates == nil || !slices.Contains(qualifiedStates, *delegationState) {
		log.Error().
			Str("staking_tx", delegation.StakingTxHashHex).
			Stringer("current_state", delegationState).
			Msg("current state is not qualified for withdrawal")
		return fmt.Errorf("current state %s is not qualified for withdrawal", *delegationState)
	}

	if err := s.emitWithdrawnDelegationEvent(ctx, delegation); err != nil {
		return fmt.Errorf("failed to emit withdrawn delegation event: %w", err)
	}

	// Update to withdrawn state
	log.Debug().
		Str("staking_tx", delegation.StakingTxHashHex).
		Stringer("state", types.StateWithdrawn).
		Stringer("sub_state", subState).
		Msg("updating delegation state to withdrawn")

	return s.db.UpdateBTCDelegationState(
		ctx,
		delegation.StakingTxHashHex,
		types.QualifiedStatesForWithdrawn(),
		types.StateWithdrawn,
		db.WithSubState(subState),
		db.WithBtcHeight(spendingHeight),
	)
}

func (s *Service) startWatchingSlashingChange(
	ctx context.Context,
	slashingTx *wire.MsgTx,
	spendingHeight uint32,
	delegation *model.BTCDelegationDetails,
	subState types.DelegationSubState,
) error {
	log.Debug().
		Str("staking_tx", delegation.StakingTxHashHex).
		Stringer("slashing_tx", slashingTx.TxHash()).
		Msg("watching for slashing change output")

	// Create outpoint for the change output (index 1)
	changeOutpoint := wire.OutPoint{
		Hash:  slashingTx.TxHash(),
		Index: 1, // Change output is always second
	}

	stakingParams, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}
	slashingChangeTimelockExpireHeight := spendingHeight + stakingParams.UnbondingTimeBlocks

	// Save timelock expire to mark it as Withdrawable when timelock expires
	// (sub state - timelock_slashing/early_unbonding_slashing)
	if err := s.db.SaveNewTimeLockExpire(
		ctx,
		delegation.StakingTxHashHex,
		slashingChangeTimelockExpireHeight,
		subState,
	); err != nil {
		return fmt.Errorf("failed to save timelock expire: %w", err)
	}

	go func() {
		// Register spend notification for the change output
		spendEv, err := s.btcNotifier.RegisterSpendNtfn(
			&changeOutpoint,
			slashingTx.TxOut[1].PkScript, // Script of change output
			delegation.StartHeight,
		)
		if err != nil {
			// TODO: Handle the error in a better way such as retrying immediately
			// If continue to fail, we could retry by sending to queue and processing
			// later again to make sure we don't miss any spend
			// Will leave it as it is for now with alerts on log
			log.Error().Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("failed to register slashing change spend notification")
			return
		}
		s.watchForSpendSlashingChange(spendEv, delegation, subState)
	}()

	return nil
}

// isSpendingStakingTxUnbondingPath checks if the transaction is spending the unbonding path
// of a staking transaction output
func (s *Service) isSpendingStakingTxUnbondingPath(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) (bool, error) {
	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}
	stakingTxHash := stakingTx.TxHash()

	// 1. an unbonding tx must be a transfer tx
	if err := btcstaking.IsTransferTx(tx); err != nil {
		return false, nil
	}

	// 2. an unbonding tx must spend the staking output
	if !tx.TxIn[0].PreviousOutPoint.Hash.IsEqual(&stakingTxHash) {
		return false, nil
	}
	if tx.TxIn[0].PreviousOutPoint.Index != delegation.StakingOutputIdx {
		return false, nil
	}

	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return false, err
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		stakingValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the staking info: %w", err)
	}
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[0].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[0].Witness[len(tx.TxIn[0].Witness)-2]

	if !bytes.Equal(unbondingPathInfo.GetPkScriptPath(), scriptFromWitness) {
		// not unbonding tx as it does not unlock the unbonding path
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("spending tx does not unlock the staking unbonding path")
		return false, nil
	}

	return true, nil
}

// validateUnbondingTxOutput validates that the output of an unbonding transaction
// matches the expected script and value according to the staking parameters
func (s *Service) validateUnbondingTxOutput(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) (bool, error) {
	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return false, err
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)

	// Validate transaction sequence and locktime
	if tx.TxIn[0].Sequence != wire.MaxTxInSequenceNum || tx.LockTime != 0 {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("unbonding tx has invalid sequence or locktime")
		return false, nil
	}

	// Calculate expected output value after fee
	unbondingFee := btcutil.Amount(params.UnbondingFeeSat)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	if expectedUnbondingOutputValue <= 0 {
		return false, fmt.Errorf("%w: staking output value is too low, got %v, unbonding fee: %v",
			types.ErrInvalidUnbondingTx, stakingValue, params.UnbondingFeeSat)
	}

	// Build expected unbonding output
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.UnbondingTime),
		expectedUnbondingOutputValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuild the unbonding info: %w", err)
	}

	// Validate output script and value
	if !bytes.Equal(tx.TxOut[0].PkScript, unbondingInfo.UnbondingOutput.PkScript) {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("unbonding tx output pk script does not match")
		return false, nil
	}
	if tx.TxOut[0].Value != unbondingInfo.UnbondingOutput.Value {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("unbonding tx output value does not match")
		return false, nil
	}

	// check if the discovered unbonding tx is the registered unbonding tx in babylon
	registeredUnbondingTxBytes, parseErr := hex.DecodeString(delegation.UnbondingTx)
	if parseErr != nil {
		return false, fmt.Errorf("failed to decode unbonding tx: %w", parseErr)
	}
	registeredUnbondingTx, parseErr := bbn.NewBTCTxFromBytes(registeredUnbondingTxBytes)
	if parseErr != nil {
		return false, fmt.Errorf("failed to parse unbonding tx: %w", parseErr)
	}
	if registeredUnbondingTx.TxHash().String() != tx.TxHash().String() {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("unbonding tx hash does not match")
		return false, nil
	}

	return true, nil
}

func (s *Service) isSpendingStakingTxTimeLockPath(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) (bool, error) {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert covenant pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return false, err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)

	// 3. re-build the timelock path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		stakingValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the staking info: %w", err)
	}

	timelockPathInfo, err := stakingInfo.TimeLockPathSpendInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(timelockPathInfo.GetPkScriptPath(), scriptFromWitness) {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("spending tx does not unlock the staking time-lock path")
		return false, nil
	}

	return true, nil
}

func (s *Service) isSpendingUnbondingTxTimeLockPath(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	spendingInputIdx uint32,
	params *bbnclient.StakingParams,
) (bool, error) {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert covenant pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return false, err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	// re-build the time-lock path script and check whether the script from
	// the witness matches
	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)
	unbondingFee := btcutil.Amount(params.UnbondingFeeSat)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.UnbondingTime),
		expectedUnbondingOutputValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the unbonding info: %w", err)
	}
	timelockPathInfo, err := unbondingInfo.TimeLockPathSpendInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(timelockPathInfo.GetPkScriptPath(), scriptFromWitness) {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("spending tx does not unlock the unbonding time-lock path")
		return false, nil
	}

	return true, nil
}

func (s *Service) isSpendingStakingTxSlashingPath(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) (bool, error) {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert covenant pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return false, err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		stakingValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the staking info: %w", err)
	}

	slashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get the slashing path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(slashingPathInfo.GetPkScriptPath(), scriptFromWitness) {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("spending tx does not unlock the staking slashing path")
		return false, nil
	}

	return true, nil
}

func (s *Service) isSpendingUnbondingTxSlashingPath(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	spendingInputIdx uint32,
	params *bbnclient.StakingParams,
) (bool, error) {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert covenant pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return false, err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	// re-build the time-lock path script and check whether the script from
	// the witness matches
	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)
	unbondingFee := btcutil.Amount(params.UnbondingFeeSat)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.UnbondingTime),
		expectedUnbondingOutputValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the unbonding info: %w", err)
	}
	slashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get the slashing path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(slashingPathInfo.GetPkScriptPath(), scriptFromWitness) {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Msg("spending tx does not unlock the unbonding slashing path")
		return false, nil
	}

	return true, nil
}
