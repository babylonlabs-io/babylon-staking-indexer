package model

import (
	"fmt"

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
	SpendingHeight                uint32 `bson:"spending_height"`
	SlashingTxHex                 string `bson:"slashing_tx_hex"`
	SlashingBTCTimestamp          int64  `bson:"slashing_btc_timestamp"`
	UnbondingSlashingTxHex        string `bson:"unbonding_slashing_tx_hex"`
	UnbondingSlashingBTCTimestamp int64  `bson:"unbonding_slashing_btc_timestamp"`
}

type StateRecord struct {
	State        types.DelegationState    `bson:"state"`
	SubState     types.DelegationSubState `bson:"sub_state,omitempty"`
	BbnHeight    int64                    `bson:"bbn_height,omitempty"` // Babylon block height when applicable
	BtcHeight    uint32                   `bson:"btc_height,omitempty"` // Bitcoin block height when applicable
	BbnEventType string                   `bson:"bbn_event_type,omitempty"`
}

type BTCDelegationDetails struct {
	StakingTxHashHex            string                       `bson:"_id"` // Primary key
	StakingTxHex                string                       `bson:"staking_tx_hex"`
	StakingTime                 uint32                       `bson:"staking_time"`
	StakingAmount               uint64                       `bson:"staking_amount"`
	StakingOutputIdx            uint32                       `bson:"staking_output_idx"`
	StakingBTCTimestamp         int64                        `bson:"staking_btc_timestamp"`
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
	UnbondingStartHeight        uint32                       `bson:"unbonding_start_height"`
	UnbondingBTCTimestamp       int64                        `bson:"unbonding_btc_timestamp"`
	CovenantUnbondingSignatures []CovenantSignature          `bson:"covenant_unbonding_signatures"`
	BTCDelegationCreatedBlock   BTCDelegationCreatedBbnBlock `bson:"btc_delegation_created_bbn_block"`
	SlashingTx                  SlashingTx                   `bson:"slashing_tx"`
}

func FromEventBTCDelegationCreated(
	event *bbntypes.EventBTCDelegationCreated,
	bbnBlockHeight,
	bbnBlockTime int64,
) (*BTCDelegationDetails, error) {
	stakingOutputIdx, err := utils.ParseUint32(event.StakingOutputIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staking output index: %w", err)
	}

	paramsVersion, err := utils.ParseUint32(event.ParamsVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse params version: %w", err)
	}

	stakingTime, err := utils.ParseUint32(event.StakingTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staking time: %w", err)
	}

	unbondingTime, err := utils.ParseUint32(event.UnbondingTime)
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
		StakingTime:                 stakingTime,
		StakingAmount:               uint64(stakingValue),
		StakingOutputIdx:            stakingOutputIdx,
		StakerBtcPkHex:              event.StakerBtcPkHex,
		FinalityProviderBtcPksHex:   event.FinalityProviderBtcPksHex,
		ParamsVersion:               paramsVersion,
		UnbondingTime:               unbondingTime,
		UnbondingTx:                 event.UnbondingTx,
		State:                       types.StatePending, // initial state will always be PENDING
		StartHeight:                 0,                  // it should be set when the inclusion proof is received
		EndHeight:                   0,                  // it should be set when the inclusion proof is received
		CovenantUnbondingSignatures: []CovenantSignature{},
		BTCDelegationCreatedBlock: BTCDelegationCreatedBbnBlock{
			Height:    bbnBlockHeight,
			Timestamp: bbnBlockTime,
		},
		StateHistory: []StateRecord{
			{
				State:        types.StatePending,
				BbnHeight:    bbnBlockHeight,
				BbnEventType: types.EventBTCDelegationCreated.ShortName(),
			},
		},
	}, nil
}

func FromEventBTCDelegationInclusionProofReceived(
	event *bbntypes.EventBTCDelegationInclusionProofReceived,
) *BTCDelegationDetails {
	startHeight, _ := utils.ParseUint32(event.StartHeight)
	endHeight, _ := utils.ParseUint32(event.EndHeight)
	return &BTCDelegationDetails{
		StartHeight: startHeight,
		EndHeight:   endHeight,
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
