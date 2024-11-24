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
	return []DelegationState{StateWithdrawable}
}

// QualifiedStatesForWithdrawable returns the qualified current states for Withdrawable event
func QualifiedStatesForWithdrawable() []DelegationState {
	return []DelegationState{StateUnbonding}
}

// QualifiedStatesForSlashedWithdrawn returns the qualified current states for SlashedWithdrawn event
func QualifiedStatesForSlashedWithdrawn() []DelegationState {
	return []DelegationState{StateSlashed}
}

type DelegationSubState string

const (
	SubStateTimelock               DelegationSubState = "TIMELOCK"
	SubStateEarlyUnbonding         DelegationSubState = "EARLY_UNBONDING"
	SubStateTimelockSlashing       DelegationSubState = "TIMELOCK_SLASHING"
	SubStateEarlyUnbondingSlashing DelegationSubState = "EARLY_UNBONDING_SLASHING"
)

func (p DelegationSubState) String() string {
	return string(p)
}
