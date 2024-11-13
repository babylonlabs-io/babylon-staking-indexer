package model

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

type BTCDelegationDetails struct {
	StakingTxHashHex          string                `bson:"_id"` // Primary key
	ParamsVersion             uint32                `bson:"params_version"`
	FinalityProviderBtcPksHex []string              `bson:"finality_provider_btc_pks_hex"`
	StakerBtcPkHex            string                `bson:"staker_btc_pk_hex"`
	StakingTime               uint32                `bson:"staking_time"`
	StakingAmount             uint64                `bson:"staking_amount"`
	StakingOutputPkScript     string                `bson:"staking_output_pk_script"`
	StakingOutputIdx          uint32                `bson:"staking_output_idx"`
	UnbondingTime             uint32                `bson:"unbonding_time"`
	UnbondingTx               string                `bson:"unbonding_tx"`
	State                     types.DelegationState `bson:"state"`
	StartHeight               uint32                `bson:"start_height"`
	EndHeight                 uint32                `bson:"end_height"`
}

func FromEventBTCDelegationCreated(
	event *bbntypes.EventBTCDelegationCreated,
) (*BTCDelegationDetails, *types.Error) {
	stakingOutputIdx, err := strconv.ParseUint(event.StakingOutputIndex, 10, 32)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking output index: %w", err),
		)
	}

	paramsVersion, err := strconv.ParseUint(event.ParamsVersion, 10, 32)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking output index: %w", err),
		)
	}

	stakingTime, err := strconv.ParseUint(event.StakingTime, 10, 32)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking output index: %w", err),
		)
	}

	stakingAmount, err := strconv.ParseUint(event.StakingAmount, 10, 32)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking output index: %w", err),
		)
	}

	unbondingTime, err := strconv.ParseUint(event.UnbondingTime, 10, 32)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking output index: %w", err),
		)
	}

	return &BTCDelegationDetails{
		StakingTxHashHex:          event.StakingTxHash, // babylon returns a hex string
		ParamsVersion:             uint32(paramsVersion),
		FinalityProviderBtcPksHex: event.FinalityProviderBtcPksHex,
		StakerBtcPkHex:            event.StakerBtcPkHex,
		StakingTime:               uint32(stakingTime),
		StakingAmount:             stakingAmount,
		StakingOutputPkScript:     event.StakingOutputPkScript,
		StakingOutputIdx:          uint32(stakingOutputIdx),
		UnbondingTime:             uint32(unbondingTime),
		UnbondingTx:               event.UnbondingTx,
		State:                     types.StatePending, // initial state will always be PENDING
		StartHeight:               uint32(0),          // it should be set when the inclusion proof is received
		EndHeight:                 uint32(0),          // it should be set when the inclusion proof is received
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
	}
}

func (d *BTCDelegationDetails) HasInclusionProof() bool {
	// Ref: https://github.com/babylonlabs-io/babylon/blob/b1a4b483f60458fcf506adf1d80aaa6c8c10f8a4/x/btcstaking/types/btc_delegation.go#L47
	return d.StartHeight > 0 && d.EndHeight > 0
}
