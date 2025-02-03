package types

type EventTypes string

type EventCategory string

func (e EventTypes) String() string {
	return string(e)
}

const (
	EventBTCDelegationCreated                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationCreated"
	EventCovenantQuorumReached               EventTypes = "babylon.btcstaking.v1.EventCovenantQuorumReached"
	EventCovenantSignatureReceived           EventTypes = "babylon.btcstaking.v1.EventCovenantSignatureReceived"
	EventBTCDelegationInclusionProofReceived EventTypes = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventTypes = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationExpired"
)

const (
	EventFinalityProviderCreatedType  EventTypes = "babylon.btcstaking.v1.EventFinalityProviderCreated"
	EventFinalityProviderEditedType   EventTypes = "babylon.btcstaking.v1.EventFinalityProviderEdited"
	EventFinalityProviderStatusChange EventTypes = "babylon.btcstaking.v1.EventFinalityProviderStatusChange"
)
