package state

import "github.com/babylonlabs-io/babylon-staking-indexer/internal/types"

// finalityProviderStateChangeMap maps the current state of a finality provider to
// the states it can transition to
// TODO: This is pending confirmed state transitions from the core team
var finalityProviderStateChangeMap = map[string][]string{
	types.FinalityProviderStateActive.String(): {
		types.ProviderStateStateInactive.String(),
		types.FinalityProviderStateJailed.String(),
		types.FinalityProviderStateSlashed.String(),
	},
	types.ProviderStateStateInactive.String(): {
		types.FinalityProviderStateActive.String(),
		types.FinalityProviderStateJailed.String(),
		types.FinalityProviderStateSlashed.String(),
	},
	types.FinalityProviderStateJailed.String(): {
		types.FinalityProviderStateActive.String(),
		types.ProviderStateStateInactive.String(),
		types.FinalityProviderStateSlashed.String(),
	},
	types.FinalityProviderStateSlashed.String(): {},
}

func IsQualifiedStateForFinalityProviderStateChange(
	currentState string, newState string,
) bool {
	qualifiedStates, ok := finalityProviderStateChangeMap[currentState]
	if !ok {
		return false
	}
	for _, state := range qualifiedStates {
		if state == newState {
			return true
		}
	}
	return false
}
