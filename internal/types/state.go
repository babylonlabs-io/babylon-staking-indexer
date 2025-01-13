package types

import bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"

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

// QualifiedStatesForWithdrawable returns the qualified current states for Withdrawable event
// The "StateWithdrawable" is included b/c sub state can be changed to if
// user did not withdraw ontime. e.g TIMELOCK change to TIMELOCK_SLASHING
func QualifiedStatesForWithdrawable() []DelegationState {
	return []DelegationState{StateUnbonding, StateSlashed, StateWithdrawable}
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
