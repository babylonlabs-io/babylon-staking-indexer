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
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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
		params, err := s.db.GetStakingParams(quitCtx, delegation.ParamsVersion)
		if err != nil {
			log.Error().Err(err).Msg("failed to get staking params")
			return
		}

		if err := s.handleSpendingStakingTransaction(
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			delegation,
			params,
		); err != nil {
			log.Error().Err(err).Msg("failed to handle spending staking transaction")
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
		params, err := s.db.GetStakingParams(quitCtx, delegation.ParamsVersion)
		if err != nil {
			log.Error().Err(err).Msg("failed to get staking params")
			return
		}

		if err := s.handleSpendingUnbondingTransaction(
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			delegation,
			params,
		); err != nil {
			log.Error().Err(err).Msg("failed to handle spending unbonding transaction")
			return
		}

	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}

}

func (s *Service) handleSpendingStakingTransaction(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) error {
	stakingTxHash, err := chainhash.NewHashFromStr(delegation.StakingTxHashHex)
	if err != nil {
		return err
	}

	// check whether it is a valid unbonding tx
	isUnbonding, err := s.IsValidUnbondingTx(tx, stakingTxHash, delegation, params)
	if err != nil {
		if errors.Is(err, types.ErrInvalidUnbondingTx) {
			return nil
		}
		return err
	}

	if !isUnbonding {
		// not an unbonding tx, so this is a withdraw tx from the staking,
		// validate it and process it
		if err := s.validateWithdrawalTxFromStaking(tx, spendingInputIdx, delegation, params); err != nil {
			if errors.Is(err, types.ErrInvalidWithdrawalTx) {
				// TODO: consider slashing transaction for phase-2
				return nil
			}
			return err
		}

		delegationState, err := s.db.GetBTCDelegationState(context.Background(), delegation.StakingTxHashHex)
		if err != nil {
			return fmt.Errorf("failed to get delegation state: %w", err)
		}

		qualifiedStates := types.QualifiedStatesForWithdrawn()
		if qualifiedStates == nil {
			return fmt.Errorf("invalid delegation state from Babylon: %s", delegationState)
		}

		if !utils.Contains(qualifiedStates, *delegationState) {
			return fmt.Errorf("current state is not qualified for transition: %s", *delegationState)
		}

		// Update delegation status
		if err := s.db.UpdateBTCDelegationState(context.Background(), delegation.StakingTxHashHex, types.StateWithdrawn); err != nil {
			return fmt.Errorf("failed to update delegation status: %w", err)
		}

		return nil
	}

	return nil
}

// IsValidUnbondingTx tries to identify a tx is a valid unbonding tx
// It returns error when (1) it fails to verify the unbonding tx due
// to invalid parameters, and (2) the tx spends the unbonding path
// but is invalid
func (s *Service) IsValidUnbondingTx(
	tx *wire.MsgTx,
	stakingTxHash *chainhash.Hash,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams) (bool, error) {
	// 1. an unbonding tx must be a transfer tx
	if err := btcstaking.IsTransferTx(tx); err != nil {
		return false, nil
	}

	// 2. an unbonding tx must spend the staking output
	//stakingTxHash := stakingTx.Tx.TxHash()
	if !tx.TxIn[0].PreviousOutPoint.Hash.IsEqual(stakingTxHash) {
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

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		btcutil.Amount(delegation.StakingAmount),
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
	stakingValue := btcutil.Amount(delegation.StakingAmount)
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

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		btcutil.Amount(delegation.StakingAmount),
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

func (s *Service) handleSpendingUnbondingTransaction(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) error {
	// Validate unbonding withdrawal transaction
	if err := s.validateWithdrawalTxFromUnbonding(tx, delegation, spendingInputIdx, params); err != nil {
		if errors.Is(err, types.ErrInvalidWithdrawalTx) {
			// TODO: consider slashing transaction for phase-2
			return nil
		}
		return err
	}

	delegationState, err := s.db.GetBTCDelegationState(context.Background(), delegation.StakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get delegation state: %w", err)
	}

	qualifiedStates := types.QualifiedStatesForWithdrawn()
	if qualifiedStates == nil || !utils.Contains(qualifiedStates, *delegationState) {
		return fmt.Errorf("current state %s is not qualified for withdrawal", *delegationState)
	}

	// Update to withdrawn state
	return s.db.UpdateBTCDelegationState(
		context.Background(),
		delegation.StakingTxHashHex,
		types.StateWithdrawn,
	)
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

	// re-build the time-lock path script and check whether the script from
	// the witness matches
	stakingValue := btcutil.Amount(delegation.StakingAmount)
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
