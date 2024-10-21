package types

// Enum values for Delegation State
type DelegationState string

const (
	StatePending   DelegationState = "PENDING"
	StateVerified  DelegationState = "VERIFIED"
	StateActive    DelegationState = "ACTIVE"
	StateUnbonding DelegationState = "UNBONDING"
	StateWithdrawn DelegationState = "WITHDRAWN"
	StateSlashed   DelegationState = "SLASHED"
	StateUnbonded  DelegationState = "UNBONDED"
)

func (s DelegationState) String() string {
	return string(s)
}
