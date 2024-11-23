package types

import bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"

// Enum values for Delegation State
// We can create a new type for more gradunal defined states.
type DelegationState string

const (
	StatePending          DelegationState = "PENDING"
	StateVerified         DelegationState = "VERIFIED"
	StateActive           DelegationState = "ACTIVE"
	StateUnbonding        DelegationState = "UNBONDING"    // TIMELOCK_UNBONDING, EARLY_UNBONDING, SLASHING_UNBONDING
	StateWithdrawable     DelegationState = "WITHDRAWABLE" // TIMELOCK_UNBONDED_WITHDRAWABLE, EARLY_UNBONDED_WITHDRAWABLE, SLASHING_UNBONDED_WITHDRAWABLE
	StateWithdrawn        DelegationState = "WITHDRAWN"    // TIMELOCK_UNBONDED_WITHDRAWN, EARLY_UNBONDED_WITHDRAWN, SLASHING_UNBONDED_WITHDRAWN
	StateSlashed          DelegationState = "SLASHED"
	StateSlashedWithdrawn DelegationState = "SLASHED_WITHDRAWN"
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
