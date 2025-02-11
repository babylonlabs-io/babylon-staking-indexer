package types

import (
	"fmt"

	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// Enum values for Delegation State
type DelegationState string

const (
	StatePending      DelegationState = "PENDING"
	StateVerified     DelegationState = "VERIFIED"
	StateActive       DelegationState = "ACTIVE"
	StateUnbonding    DelegationState = "UNBONDING"
	StateWithdrawable DelegationState = "WITHDRAWABLE"
	StateWithdrawn    DelegationState = "WITHDRAWN"
	StateSlashed      DelegationState = "SLASHED"
)

func (s DelegationState) String() string {
	return string(s)
}

// QualifiedStatesForCovenantQuorumReached returns the qualified current states for CovenantQuorumReached event
func QualifiedStatesForCovenantQuorumReached(babylonState string) []DelegationState {
	switch babylonState {
	case bbntypes.BTCDelegationStatus_VERIFIED.String(), bbntypes.BTCDelegationStatus_ACTIVE.String():
		return []DelegationState{StatePending}
	default:
		return nil
	}
}

// QualifiedStatesForInclusionProofReceived returns the qualified current states for InclusionProofReceived event
func QualifiedStatesForInclusionProofReceived(babylonState string) []DelegationState {
	switch babylonState {
	case bbntypes.BTCDelegationStatus_ACTIVE.String():
		return []DelegationState{StateVerified}
	case bbntypes.BTCDelegationStatus_PENDING.String():
		return []DelegationState{StatePending}
	default:
		return nil
	}
}

// QualifiedStatesForUnbondedEarly returns the qualified current states for UnbondedEarly event
func QualifiedStatesForUnbondedEarly() []DelegationState {
	return []DelegationState{StateActive}
}

// QualifiedStatesForExpired returns the qualified current states for Expired event
func QualifiedStatesForExpired() []DelegationState {
	return []DelegationState{StateActive}
}

// QualifiedStatesForWithdrawn returns the qualified current states for Withdrawn event
func QualifiedStatesForWithdrawn() []DelegationState {
	// StateActive/StateUnbonding/StateSlashed is included b/c its possible that expiry checker
	// or babylon notifications are slow and in meanwhile the btc subscription encounters
	// the spending/withdrawal tx
	return []DelegationState{StateActive, StateUnbonding, StateWithdrawable, StateSlashed}
}

// QualifiedStatesForWithdrawable returns the qualified states that can transition to Withdrawable
// based on the delegation's sub-state.
func QualifiedStatesForWithdrawable(subState DelegationSubState) ([]DelegationState, error) {
	switch subState {
	case SubStateEarlyUnbonding, SubStateTimelock:
		// For normal unbonding flows (early unbonding or timelock expiry),
		// we expect the delegation to be in the Unbonding state.
		// State transition: Active -> Unbonding -> Withdrawable
		return []DelegationState{StateUnbonding}, nil

	case SubStateTimelockSlashing, SubStateEarlyUnbondingSlashing:
		// For slashing flows, we expect the delegation to be in the Slashed state.
		// This handles multiple scenarios:
		// 1. Active -> Slashed -> Withdrawable
		// 2. Active -> Unbonding -> Slashed -> Withdrawable
		// 3. Active -> Unbonding -> Withdrawable -> Slashed -> Withdrawable
		//    (SubState transitions from Timelock -> TimelockSlashing or
		//     EarlyUnbonding -> EarlyUnbondingSlashing)
		return []DelegationState{StateSlashed}, nil

	default:
		return nil, fmt.Errorf("unknown delegation sub state: %s", subState)
	}
}

// QualifiedStatesForSlashed returns the qualified current states for Slashed transition
func QualifiedStatesForSlashed() []DelegationState {
	return []DelegationState{StateActive, StateUnbonding, StateWithdrawable}
}

type DelegationSubState string

const (
	SubStateTimelock       DelegationSubState = "TIMELOCK"
	SubStateEarlyUnbonding DelegationSubState = "EARLY_UNBONDING"

	// Used only for Withdrawable and Withdrawn parent states
	SubStateTimelockSlashing       DelegationSubState = "TIMELOCK_SLASHING"
	SubStateEarlyUnbondingSlashing DelegationSubState = "EARLY_UNBONDING_SLASHING"
)

func (p DelegationSubState) String() string {
	return string(p)
}
