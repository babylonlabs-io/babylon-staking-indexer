package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
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
			Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
			Msg("staking tx has been spent")
		if err := s.handleSpendingStakingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			uint32(spendDetail.SpendingHeight),
			delegation,
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
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
			Str("unbonding_tx", spendDetail.SpendingTx.TxHash().String()).
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
				Str("unbonding_tx", spendDetail.SpendingTx.TxHash().String()).
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
			Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
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
		if qualifiedStates == nil || !utils.Contains(qualifiedStates, *delegationState) {
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("state", delegationState.String()).
				Msg("current state is not qualified for slashed withdrawn")
			return
		}

		// Update to withdrawn state
		delegationSubState := subState
		if err := s.db.UpdateBTCDelegationState(
			quitCtx,
			delegation.StakingTxHashHex,
			types.QualifiedStatesForWithdrawn(),
			types.StateWithdrawn,
			&delegationSubState,
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("state", types.StateWithdrawn.String()).
				Str("sub_state", delegationSubState.String()).
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
	delegation *model.BTCDelegationDetails,
) error {
	params, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	// First try to validate as unbonding tx
	isUnbonding, err := s.IsValidUnbondingTx(spendingTx, delegation, params)
	if err != nil {
		return fmt.Errorf("failed to validate unbonding tx: %w", err)
	}
	if isUnbonding {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("unbonding_tx", spendingTx.TxHash().String()).
			Msg("staking tx has been spent through unbonding path")

		// Register unbonding spend notification
		return s.registerUnbondingSpendNotification(ctx, delegation)
	}

	// Try to validate as withdrawal transaction
	withdrawalErr := s.validateWithdrawalTxFromStaking(spendingTx, spendingInputIdx, delegation, params)
	if withdrawalErr == nil {
		// It's a valid withdrawal, process it
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("withdrawal_tx", spendingTx.TxHash().String()).
			Msg("staking tx has been spent through withdrawal path")
		return s.handleWithdrawal(ctx, delegation, types.SubStateTimelock)
	}

	// If it's not a valid withdrawal, check if it's a valid slashing
	if !errors.Is(withdrawalErr, types.ErrInvalidWithdrawalTx) {
		return fmt.Errorf("failed to validate withdrawal tx: %w", withdrawalErr)
	}

	// Try to validate as slashing transaction
	if err := s.validateSlashingTxFromStaking(spendingTx, spendingInputIdx, delegation, params); err != nil {
		if errors.Is(err, types.ErrInvalidSlashingTx) {
			// Neither withdrawal nor slashing - this is an invalid spend
			return fmt.Errorf("transaction is neither valid unbonding, withdrawal, nor slashing: %w", err)
		}
		return fmt.Errorf("failed to validate slashing tx: %w", err)
	}

	// Save slashing tx hex
	slashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(spendingTx)
	if err != nil {
		return fmt.Errorf("failed to convert slashing tx to bytes: %w", err)
	}
	slashingTxHex := slashingTx.ToHexStr()
	if err := s.db.SaveBTCDelegationSlashingTxHex(ctx, delegation.StakingTxHashHex, slashingTxHex); err != nil {
		return fmt.Errorf("failed to save slashing tx hex: %w", err)
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
	withdrawalErr := s.validateWithdrawalTxFromUnbonding(spendingTx, delegation, spendingInputIdx, params)
	if withdrawalErr == nil {
		// It's a valid withdrawal, process it
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("unbonding_tx", spendingTx.TxHash().String()).
			Msg("unbonding tx has been spent through withdrawal path")
		return s.handleWithdrawal(ctx, delegation, types.SubStateEarlyUnbonding)
	}

	// If it's not a valid withdrawal, check if it's a valid slashing
	if !errors.Is(withdrawalErr, types.ErrInvalidWithdrawalTx) {
		return fmt.Errorf("failed to validate withdrawal tx: %w", withdrawalErr)
	}

	// Try to validate as slashing transaction
	if err := s.validateSlashingTxFromUnbonding(spendingTx, delegation, spendingInputIdx, params); err != nil {
		if errors.Is(err, types.ErrInvalidSlashingTx) {
			// Neither withdrawal nor slashing - this is an invalid spend
			return fmt.Errorf("transaction is neither valid withdrawal nor slashing: %w", err)
		}
		return fmt.Errorf("failed to validate slashing tx: %w", err)
	}

	// Save unbonding slashing tx hex
	unbondingSlashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(spendingTx)
	if err != nil {
		return fmt.Errorf("failed to convert unbonding slashing tx to bytes: %w", err)
	}
	unbondingSlashingTxHex := unbondingSlashingTx.ToHexStr()
	if err := s.db.SaveBTCDelegationUnbondingSlashingTxHex(ctx, delegation.StakingTxHashHex, unbondingSlashingTxHex); err != nil {
		return fmt.Errorf("failed to save unbonding slashing tx hex: %w", err)
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

func (s *Service) handleWithdrawal(
	ctx context.Context,
	delegation *model.BTCDelegationDetails,
	subState types.DelegationSubState,
) error {
	delegationState, err := s.db.GetBTCDelegationState(ctx, delegation.StakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get delegation state: %w", err)
	}

	qualifiedStates := types.QualifiedStatesForWithdrawn()
	if qualifiedStates == nil || !utils.Contains(qualifiedStates, *delegationState) {
		log.Error().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("current_state", delegationState.String()).
			Msg("current state is not qualified for withdrawal")
		return fmt.Errorf("current state %s is not qualified for withdrawal", *delegationState)
	}

	// Update to withdrawn state
	log.Debug().
		Str("staking_tx", delegation.StakingTxHashHex).
		Str("state", types.StateWithdrawn.String()).
		Str("sub_state", subState.String()).
		Msg("updating delegation state to withdrawn")
	return s.db.UpdateBTCDelegationState(
		ctx,
		delegation.StakingTxHashHex,
		types.QualifiedStatesForWithdrawn(),
		types.StateWithdrawn,
		&subState,
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
		Str("slashing_tx", slashingTx.TxHash().String()).
		Msg("watching for slashing change output")

	// Create outpoint for the change output (index 1)
	changeOutpoint := wire.OutPoint{
		Hash:  slashingTx.TxHash(),
		Index: 1, // Change output is always second
	}

	// Register spend notification for the change output
	spendEv, err := s.btcNotifier.RegisterSpendNtfn(
		&changeOutpoint,
		slashingTx.TxOut[1].PkScript, // Script of change output
		delegation.StartHeight,
	)
	if err != nil {
		return fmt.Errorf("failed to register spend ntfn for slashing change output: %w", err)
	}

	stakingParams, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}
	slashingChangeTimelockExpireHeight := spendingHeight + stakingParams.MinUnbondingTimeBlocks

	// Save timelock expire to mark it as Withdrawn (sub state - timelock_slashing/early_unbonding_slashing)
	if err := s.db.SaveNewTimeLockExpire(
		ctx,
		delegation.StakingTxHashHex,
		slashingChangeTimelockExpireHeight,
		subState,
	); err != nil {
		return fmt.Errorf("failed to save timelock expire: %w", err)
	}

	s.wg.Add(1)
	go s.watchForSpendSlashingChange(spendEv, delegation, subState)

	return nil
}

// IsValidUnbondingTx tries to identify a tx is a valid unbonding tx
// It returns error when (1) it fails to verify the unbonding tx due
// to invalid parameters, and (2) the tx spends the unbonding path
// but is invalid
func (s *Service) IsValidUnbondingTx(
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
		return false, nil
	}

	// 4. check whether the unbonding tx enables rbf has time lock
	if tx.TxIn[0].Sequence != wire.MaxTxInSequenceNum {
		return false, fmt.Errorf("%w: unbonding tx should not enable rbf", types.ErrInvalidUnbondingTx)
	}
	if tx.LockTime != 0 {
		return false, fmt.Errorf("%w: unbonding tx should not set lock time", types.ErrInvalidUnbondingTx)
	}

	// 5. check whether the script of an unbonding tx output is expected
	// by re-building unbonding output from params
	unbondingFee := btcutil.Amount(params.UnbondingFeeSat)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	if expectedUnbondingOutputValue <= 0 {
		return false, fmt.Errorf("%w: staking output value is too low, got %v, unbonding fee: %v",
			types.ErrInvalidUnbondingTx, stakingValue, params.UnbondingFeeSat)
	}
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
	if !bytes.Equal(tx.TxOut[0].PkScript, unbondingInfo.UnbondingOutput.PkScript) {
		return false, fmt.Errorf("%w: the unbonding output is not expected", types.ErrInvalidUnbondingTx)
	}
	if tx.TxOut[0].Value != unbondingInfo.UnbondingOutput.Value {
		return false, fmt.Errorf("%w: the unbonding output value %d is not expected %d",
			types.ErrInvalidUnbondingTx, tx.TxOut[0].Value, unbondingInfo.UnbondingOutput.Value)
	}

	return true, nil
}

func (s *Service) validateWithdrawalTxFromStaking(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
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
		return fmt.Errorf("failed to rebuid the staking info: %w", err)
	}

	timelockPathInfo, err := stakingInfo.TimeLockPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(timelockPathInfo.GetPkScriptPath(), scriptFromWitness) {
		return fmt.Errorf("%w: the tx does not unlock the time-lock path", types.ErrInvalidWithdrawalTx)
	}

	return nil
}

func (s *Service) validateWithdrawalTxFromUnbonding(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	spendingInputIdx uint32,
	params *bbnclient.StakingParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
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
		return fmt.Errorf("failed to rebuid the unbonding info: %w", err)
	}
	timelockPathInfo, err := unbondingInfo.TimeLockPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(timelockPathInfo.GetPkScriptPath(), scriptFromWitness) {
		return fmt.Errorf("%w: the tx does not unlock the time-lock path", types.ErrInvalidWithdrawalTx)
	}

	return nil
}

func (s *Service) validateSlashingTxFromStaking(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
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
		return fmt.Errorf("failed to rebuid the staking info: %w", err)
	}

	slashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get the slashing path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(slashingPathInfo.GetPkScriptPath(), scriptFromWitness) {
		return fmt.Errorf("%w: the tx does not unlock the slashing path", types.ErrInvalidSlashingTx)
	}

	return nil
}

func (s *Service) validateSlashingTxFromUnbonding(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	spendingInputIdx uint32,
	params *bbnclient.StakingParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams, err := utils.GetBTCParams(s.cfg.BTC.NetParams)
	if err != nil {
		return err
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
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
		return fmt.Errorf("failed to rebuid the unbonding info: %w", err)
	}
	slashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get the slashing path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(slashingPathInfo.GetPkScriptPath(), scriptFromWitness) {
		return fmt.Errorf("%w: the tx does not unlock the slashing path", types.ErrInvalidSlashingTx)
	}

	return nil
}
