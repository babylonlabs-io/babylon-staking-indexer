package types

import "fmt"

type FinalityProviderState string

const (
	FinalityProviderStateActive  FinalityProviderState = "active"
	ProviderStateStateInactive   FinalityProviderState = "inactive"
	FinalityProviderStateJailed  FinalityProviderState = "jailed"
	FinalityProviderStateSlashed FinalityProviderState = "slashed"
)

func (s FinalityProviderState) String() string {
	return string(s)
}

func FromString(s string) (FinalityProviderState, error) {
	switch s {
	case "active":
		return FinalityProviderStateActive, nil
	case "inactive":
		return ProviderStateStateInactive, nil
	case "jailed":
		return FinalityProviderStateJailed, nil
	case "slashed":
		return FinalityProviderStateSlashed, nil
	default:
		return "", fmt.Errorf("invalid finality provider state: %s", s)
	}
}
