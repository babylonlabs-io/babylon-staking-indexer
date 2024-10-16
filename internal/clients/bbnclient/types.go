package bbnclient

import (
	"encoding/hex"

	checkpointtypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	stakingtypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// StakingParams represents the staking parameters of the BBN chain
// Reference: https://github.com/babylonlabs-io/babylon/blob/main/proto/babylon/btcstaking/v1/params.proto
type StakingParams struct {
	CovenantPks                  []string
	CovenantQuorum               uint32
	MinStakingValueSat           int64
	MaxStakingValueSat           int64
	MinStakingTimeBlocks         uint32
	MaxStakingTimeBlocks         uint32
	SlashingPkScript             string
	MinSlashingTxFeeSat          int64
	SlashingRate                 string
	MinUnbondingTimeBlocks       uint32
	UnbondingFeeSat              int64
	MinCommissionRate            string
	MaxActiveFinalityProviders   uint32
	DelegationCreationBaseGasFee uint64
}

func FromBbnStakingParams(params stakingtypes.Params) *StakingParams {
	return &StakingParams{
		CovenantPks:                  params.CovenantPksHex(),
		CovenantQuorum:               params.CovenantQuorum,
		MinStakingValueSat:           params.MinStakingValueSat,
		MaxStakingValueSat:           params.MaxStakingValueSat,
		MinStakingTimeBlocks:         params.MinStakingTimeBlocks,
		MaxStakingTimeBlocks:         params.MaxStakingTimeBlocks,
		SlashingPkScript:             hex.EncodeToString(params.SlashingPkScript),
		MinSlashingTxFeeSat:          params.MinSlashingTxFeeSat,
		SlashingRate:                 params.SlashingRate.String(),
		MinUnbondingTimeBlocks:       params.MinUnbondingTimeBlocks,
		UnbondingFeeSat:              params.UnbondingFeeSat,
		MinCommissionRate:            params.MinCommissionRate.String(),
		MaxActiveFinalityProviders:   params.MaxActiveFinalityProviders,
		DelegationCreationBaseGasFee: params.DelegationCreationBaseGasFee,
	}
}

type CheckpointParams = checkpointtypes.Params
