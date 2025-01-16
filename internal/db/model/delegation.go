package model

import (
	"fmt"
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcutil"
)

type CovenantSignature struct {
	CovenantBtcPkHex string `bson:"covenant_btc_pk_hex"`
	SignatureHex     string `bson:"signature_hex"`
}

type BTCDelegationCreatedBbnBlock struct {
	Height    int64 `bson:"height"`
	Timestamp int64 `bson:"timestamp"` // epoch time in seconds
}

type SlashingTx struct {
	SlashingTxHex          string `bson:"slashing_tx_hex"`
	UnbondingSlashingTxHex string `bson:"unbonding_slashing_tx_hex"`
	SpendingHeight         uint32 `bson:"spending_height"`
}

type StateRecord struct {
	State     types.DelegationState    `bson:"state"`
	SubState  types.DelegationSubState `bson:"sub_state,omitempty"`
	BbnHeight int64                    `bson:"bbn_height,omitempty"` // Babylon block height when applicable
	BtcHeight int64                    `bson:"btc_height,omitempty"` // Bitcoin block height when applicable
}

type BTCDelegationDetails struct {
	StakingTxHashHex            string                       `bson:"_id"` // Primary key
	StakingTxHex                string                       `bson:"staking_tx_hex"`
	StakingTime                 uint32                       `bson:"staking_time"`
	StakingAmount               uint64                       `bson:"staking_amount"`
	StakingOutputIdx            uint32                       `bson:"staking_output_idx"`
	StakerBtcPkHex              string                       `bson:"staker_btc_pk_hex"`
	FinalityProviderBtcPksHex   []string                     `bson:"finality_provider_btc_pks_hex"`
	StartHeight                 uint32                       `bson:"start_height"`
	EndHeight                   uint32                       `bson:"end_height"`
	State                       types.DelegationState        `bson:"state"`
	SubState                    types.DelegationSubState     `bson:"sub_state,omitempty"`
	StateHistory                []StateRecord                `bson:"state_history"`
	ParamsVersion               uint32                       `bson:"params_version"`
	UnbondingTime               uint32                       `bson:"unbonding_time"`
	UnbondingTx                 string                       `bson:"unbonding_tx"`
	CovenantUnbondingSignatures []CovenantSignature          `bson:"covenant_unbonding_signatures"`
	BTCDelegationCreatedBlock   BTCDelegationCreatedBbnBlock `bson:"btc_delegation_created_bbn_block"`
	SlashingTx                  SlashingTx                   `bson:"slashing_tx"`
}

func FromEventBTCDelegationCreated(
	event *bbntypes.EventBTCDelegationCreated,
	bbnBlockHeight,
	bbnBlockTime int64,
) (*BTCDelegationDetails, error) {
	stakingOutputIdx, err := strconv.ParseUint(event.StakingOutputIndex, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staking output index: %w", err)
	}

	paramsVersion, err := strconv.ParseUint(event.ParamsVersion, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse params version: %w", err)
	}

	stakingTime, err := strconv.ParseUint(event.StakingTime, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staking time: %w", err)
	}

	unbondingTime, err := strconv.ParseUint(event.UnbondingTime, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unbonding time: %w", err)
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(event.StakingTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[stakingOutputIdx].Value)

	return &BTCDelegationDetails{
		StakingTxHashHex:            stakingTx.TxHash().String(),
		StakingTxHex:                event.StakingTxHex,
		StakingTime:                 uint32(stakingTime),
		StakingAmount:               uint64(stakingValue),
		StakingOutputIdx:            uint32(stakingOutputIdx),
		StakerBtcPkHex:              event.StakerBtcPkHex,
		FinalityProviderBtcPksHex:   event.FinalityProviderBtcPksHex,
		ParamsVersion:               uint32(paramsVersion),
		UnbondingTime:               uint32(unbondingTime),
		UnbondingTx:                 event.UnbondingTx,
		State:                       types.StatePending, // initial state will always be PENDING
		StartHeight:                 uint32(0),          // it should be set when the inclusion proof is received
		EndHeight:                   uint32(0),          // it should be set when the inclusion proof is received
		CovenantUnbondingSignatures: []CovenantSignature{},
		BTCDelegationCreatedBlock: BTCDelegationCreatedBbnBlock{
			Height:    bbnBlockHeight,
			Timestamp: bbnBlockTime,
		},
		StateHistory: []StateRecord{
			{
				State:     types.StatePending,
				BbnHeight: bbnBlockHeight,
			},
		},
	}, nil
}

func FromEventBTCDelegationInclusionProofReceived(
	event *bbntypes.EventBTCDelegationInclusionProofReceived,
) *BTCDelegationDetails {
	startHeight, _ := strconv.ParseUint(event.StartHeight, 10, 32)
	endHeight, _ := strconv.ParseUint(event.EndHeight, 10, 32)
	return &BTCDelegationDetails{
		StartHeight: uint32(startHeight),
		EndHeight:   uint32(endHeight),
		State:       types.DelegationState(event.NewState),
	}
}

func (d *BTCDelegationDetails) HasInclusionProof() bool {
	// Ref: https://github.com/babylonlabs-io/babylon/blob/b1a4b483f60458fcf506adf1d80aaa6c8c10f8a4/x/btcstaking/types/btc_delegation.go#L47
	return d.StartHeight > 0 && d.EndHeight > 0
}

func ToStateStrings(stateHistory []StateRecord) []string {
	states := make([]string, len(stateHistory))
	for i, record := range stateHistory {
		states[i] = record.State.String()
	}
	return states
}
