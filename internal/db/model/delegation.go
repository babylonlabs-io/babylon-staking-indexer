package model

import (
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

type BTCDelegationDetails struct {
	StakingTxHashHex          string                `bson:"_id"` // Primary key
	ParamsVersion             string                `bson:"params_version"`
	FinalityProviderBtcPksHex []string              `bson:"finality_provider_btc_pks_hex"`
	StakerBtcPkHex            string                `bson:"staker_btc_pk_hex"`
	StakingTime               string                `bson:"staking_time"`
	StakingAmount             string                `bson:"staking_amount"`
	UnbondingTime             string                `bson:"unbonding_time"`
	UnbondingTx               string                `bson:"unbonding_tx"`
	State                     types.DelegationState `bson:"state"`
	StartHeight               uint32                `bson:"start_height"`
	EndHeight                 uint32                `bson:"end_height"`
}

func FromEventBTCDelegationCreated(
	event *bbntypes.EventBTCDelegationCreated,
) *BTCDelegationDetails {
	return &BTCDelegationDetails{
		StakingTxHashHex:          event.StakingTxHash, // babylon returns a hex string
		ParamsVersion:             event.ParamsVersion,
		FinalityProviderBtcPksHex: event.FinalityProviderBtcPksHex,
		StakerBtcPkHex:            event.StakerBtcPkHex,
		StakingTime:               event.StakingTime,
		StakingAmount:             event.StakingAmount,
		UnbondingTime:             event.UnbondingTime,
		UnbondingTx:               event.UnbondingTx,
		State:                     types.StatePending, // initial state will always be PENDING
		StartHeight:               uint32(0),          // it should be set when the inclusion proof is received
		EndHeight:                 uint32(0),          // it should be set when the inclusion proof is received
	}
}

func FromEventBTCDelegationInclusionProofReceived(
	event *bbntypes.EventBTCDelegationInclusionProofReceived,
) *BTCDelegationDetails {
	startHeight, _ := strconv.ParseUint(event.StartHeight, 10, 32)
	endHeight, _ := strconv.ParseUint(event.EndHeight, 10, 32)
	return &BTCDelegationDetails{
		StartHeight: uint32(startHeight),
		EndHeight:   uint32(endHeight),
	}
}

func (d *BTCDelegationDetails) HasInclusionProof() bool {
	// Ref: https://github.com/babylonlabs-io/babylon/blob/b1a4b483f60458fcf506adf1d80aaa6c8c10f8a4/x/btcstaking/types/btc_delegation.go#L47
	return d.StartHeight > 0 && d.EndHeight > 0
}
