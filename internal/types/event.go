package types

type EventType string

type EventCategory string

func (e EventType) String() string {
	return string(e)
}

const (
	EventBTCDelegationCreated                EventType = "babylon.btcstaking.v1.EventBTCDelegationCreated"
	EventCovenantQuorumReached               EventType = "babylon.btcstaking.v1.EventCovenantQuorumReached"
	EventCovenantSignatureReceived           EventType = "babylon.btcstaking.v1.EventCovenantSignatureReceived"
	EventBTCDelegationInclusionProofReceived EventType = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventType = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventType = "babylon.btcstaking.v1.EventBTCDelegationExpired"
)

const (
	EventFinalityProviderCreatedType  EventType = "babylon.btcstaking.v1.EventFinalityProviderCreated"
	EventFinalityProviderEditedType   EventType = "babylon.btcstaking.v1.EventFinalityProviderEdited"
	EventFinalityProviderStatusChange EventType = "babylon.btcstaking.v1.EventFinalityProviderStatusChange"
)
