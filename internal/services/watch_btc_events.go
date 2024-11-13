package services

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

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
)

func (s *Service) watchForSpend(
	spendEvent *notifier.SpendEvent,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams) {
	defer s.wg.Done()
	quitCtx, cancel := s.quitContext()
	defer cancel()

	var (
		spendingTx       *wire.MsgTx = nil
		spendingInputIdx uint32      = 0
	)
	select {
	case spendDetail := <-spendEvent.Spend:
		spendingTx = spendDetail.SpendingTx
		spendingInputIdx = spendDetail.SpenderInputIndex
	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}

	err := s.handleSpendingStakingTransaction(
		spendingTx,
		spendingInputIdx,
		delegation,
		params)
	if err != nil {
		panic(err)
	}
}

func (s *Service) handleSpendingStakingTransaction(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) error {
	stakingTxHash, parseErr := chainhash.NewHashFromStr(delegation.StakingTxHashHex)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking tx hash: %w", parseErr),
		)
	}

	// check whether it is a valid unbonding tx
	isUnbonding, err := s.IsValidUnbondingTx(tx, stakingTxHash, delegation, params)
	if err != nil {
		if errors.Is(err, types.ErrInvalidUnbondingTx) {
			//invalidTransactionsCounter.WithLabelValues("confirmed_unbonding_transactions").Inc()
			//si.logger.Warn("found an invalid unbonding tx",
			//	zap.String("tx_hash", tx.TxHash().String()),
			//	zap.Uint64("height", height),
			//	zap.Bool("is_confirmed", true),
			//	zap.Error(err),
			//)

			return nil
		}
		// record metrics
		//failedVerifyingUnbondingTxsCounter.Inc()
		return err
	}

	if !isUnbonding {
		// not an unbonding tx, so this is a withdraw tx from the staking,
		// validate it and process it
		if err := s.ValidateWithdrawalTxFromStaking(tx, spendingInputIdx, delegation, params); err != nil {
			if errors.Is(err, types.ErrInvalidWithdrawalTx) {
				//invalidTransactionsCounter.WithLabelValues("confirmed_withdraw_staking_transactions").Inc()
				//si.logger.Warn("found an invalid withdrawal tx from staking",
				//	zap.String("tx_hash", tx.TxHash().String()),
				//	zap.Uint64("height", height),
				//	zap.Bool("is_confirmed", true),
				//	zap.Error(err),
				//)

				return nil
			}

			//failedProcessingWithdrawTxsFromStakingCounter.Inc()
			return err
		}
		//if err := s.processWithdrawTx(tx, &stakingTxHash, nil, height); err != nil {
		//	// record metrics
		//	//failedProcessingWithdrawTxsFromStakingCounter.Inc()
		//
		//	return err
		//}
		return nil
	}

	// 5. this is a valid unbonding tx, process it
	//if err := s.ProcessUnbondingTx(
	//	tx, stakingTxHash, height, timestamp,
	//	paramsFromStakingTxHeight,
	//); err != nil {
	//	if !errors.Is(err, indexerstore.ErrDuplicateTransaction) {
	//		// record metrics
	//		failedProcessingUnbondingTxsCounter.Inc()
	//
	//		return err
	//	}
	//	// we don't consider duplicate error critical as it can happen
	//	// when the indexer restarts
	//	si.logger.Warn("found a duplicate tx",
	//		zap.String("tx_hash", tx.TxHash().String()))
	//}

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

func (s *Service) ValidateWithdrawalTxFromStaking(
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

//func (s *Service) processWithdrawTx(tx *wire.MsgTx, stakingTxHash *chainhash.Hash, unbondingTxHash *chainhash.Hash, height uint64) error {
//	txHashHex := tx.TxHash().String()
//	if unbondingTxHash == nil {
//		s.logger.Info("found a withdraw tx from staking",
//			zap.String("tx_hash", txHashHex),
//			zap.String("staking_tx_hash", stakingTxHash.String()),
//		)
//	} else {
//		s.logger.Info("found a withdraw tx from unbonding",
//			zap.String("tx_hash", txHashHex),
//			zap.String("staking_tx_hash", stakingTxHash.String()),
//			zap.String("unbonding_tx_hash", unbondingTxHash.String()),
//		)
//	}
//
//	withdrawEvent := queuecli.NewWithdrawStakingEvent(stakingTxHash.String())
//
//	if err := si.consumer.PushWithdrawEvent(&withdrawEvent); err != nil {
//		return fmt.Errorf("failed to push the withdraw event to the consumer: %w", err)
//	}
//
//	return nil
//}

//func (s *Service) ProcessUnbondingTx(
//	tx *wire.MsgTx,
//	stakingTxHash *chainhash.Hash,
//	params *bbnclient.StakingParams,
//) error {
//	//si.logger.Info("found an unbonding tx",
//	//	zap.Uint64("height", height),
//	//	zap.String("tx_hash", tx.TxHash().String()),
//	//	zap.String("staking_tx_hash", stakingTxHash.String()),
//	//)
//
//	unbondingTxHex, err := getTxHex(tx)
//	if err != nil {
//		return err
//	}
//
//	unbondingTxHash := tx.TxHash()
//	//unbondingEvent := queuecli.NewUnbondingStakingEvent(
//	//	stakingTxHash.String(),
//	//	height,
//	//	timestamp.Unix(),
//	//	uint64(params.UnbondingTime),
//	//	// valid unbonding tx always has one output
//	//	0,
//	//	unbondingTxHex,
//	//	unbondingTxHash.String(),
//	//)
//
//	if err := si.consumer.PushUnbondingEvent(&unbondingEvent); err != nil {
//		return fmt.Errorf("failed to push the unbonding event to the queue: %w", err)
//	}
//
//	si.logger.Info("saving the unbonding tx",
//		zap.String("tx_hash", unbondingTxHash.String()))
//
//	if err := si.is.AddUnbondingTransaction(
//		tx,
//		stakingTxHash,
//	); err != nil && !errors.Is(err, indexerstore.ErrDuplicateTransaction) {
//		return fmt.Errorf("failed to add the unbonding tx to store: %w", err)
//	}
//
//	si.logger.Info("successfully saved the unbonding tx",
//		zap.String("tx_hash", tx.TxHash().String()))
//
//	// record metrics
//	totalUnbondingTxs.Inc()
//	lastFoundUnbondingTxHeight.Set(float64(height))
//
//	return nil
//}

func (s *Service) handleSpendingUnbondingTransaction(
	tx *wire.MsgTx,
	spendingInputIdx int,
	delegation *model.BTCDelegationDetails,
	params *bbnclient.StakingParams,
) error {
	// get the stored staking tx for later validation
	//storedStakingTx, err := si.GetStakingTxByHash(unbondingTx.StakingTxHash)
	//if err != nil {
	//	// record metrics
	//	//failedProcessingWithdrawTxsFromUnbondingCounter.Inc()
	//
	//	return err
	//}

	//stakingTxHash, parseErr := chainhash.NewHashFromStr(delegation.StakingTxHashHex)
	//if parseErr != nil {
	//	return types.NewError(
	//		http.StatusInternalServerError,
	//		types.InternalServiceError,
	//		fmt.Errorf("failed to parse staking tx hash: %w", parseErr),
	//	)
	//}

	if err := s.ValidateWithdrawalTxFromUnbonding(tx, delegation, spendingInputIdx, params); err != nil {
		if errors.Is(err, types.ErrInvalidWithdrawalTx) {
			// TODO consider slashing transaction for phase-2
			//invalidTransactionsCounter.WithLabelValues("confirmed_withdraw_unbonding_transactions").Inc()
			//si.logger.Warn("found an invalid withdrawal tx from unbonding",
			//	zap.String("tx_hash", tx.TxHash().String()),
			//	zap.Uint64("height", height),
			//	zap.Bool("is_confirmed", true),
			//	zap.Error(err),
			//)

			return nil
		}

		//failedProcessingWithdrawTxsFromUnbondingCounter.Inc()
		return err
	}

	//unbondingTxHash := unbondingTx.Tx.TxHash()
	//if err := s.processWithdrawTx(tx, unbondingTx.StakingTxHash, &unbondingTxHash, height); err != nil {
	//	// record metrics
	//	failedProcessingWithdrawTxsFromUnbondingCounter.Inc()
	//
	//	return err
	//}

	return nil
}

func (s *Service) ValidateWithdrawalTxFromUnbonding(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	spendingInputIdx int,
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
