package model

import (
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
		State:                     types.DelegationState(event.NewState),
	}
}
