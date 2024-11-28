package client

const (
	ActiveStakingQueueName    string = "active_staking_queue"
	UnbondingStakingQueueName string = "unbonding_staking_queue"
)

const (
	ActiveStakingEventType    EventType = 1
	UnbondingStakingEventType EventType = 2
)

// Event schema versions, only increment when the schema changes
const (
	ActiveEventVersion    int = 0
	UnbondingEventVersion int = 0
)

type EventType int

type EventMessage interface {
	GetEventType() EventType
	GetStakingTxHashHex() string
}

type StakingEvent struct {
	SchemaVersion             int       `json:"schema_version"`
	EventType                 EventType `json:"event_type"`
	StakingTxHashHex          string    `json:"staking_tx_hash_hex"`
	StakerBtcPkHex            string    `json:"staker_btc_pk_hex"`
	FinalityProviderBtcPksHex []string  `json:"finality_provider_btc_pks_hex"`
	StakingAmount             uint64    `json:"staking_amount"`
}

func (e StakingEvent) GetEventType() EventType {
	return e.EventType
}

func (e StakingEvent) GetStakingTxHashHex() string {
	return e.StakingTxHashHex
}

func NewActiveStakingEvent(
	stakingTxHashHex string,
	stakerBtcPkHex string,
	finalityProviderBtcPksHex []string,
	stakingAmount uint64,
) StakingEvent {
	return StakingEvent{
		SchemaVersion:             ActiveEventVersion,
		EventType:                 ActiveStakingEventType,
		StakingTxHashHex:          stakingTxHashHex,
		StakerBtcPkHex:            stakerBtcPkHex,
		FinalityProviderBtcPksHex: finalityProviderBtcPksHex,
		StakingAmount:             stakingAmount,
	}
}

func NewUnbondingStakingEvent(
	stakingTxHashHex string,
	stakerBtcPkHex string,
	finalityProviderBtcPksHex []string,
	stakingAmount uint64,
) StakingEvent {
	return StakingEvent{
		SchemaVersion:             UnbondingEventVersion,
		EventType:                 UnbondingStakingEventType,
		StakingTxHashHex:          stakingTxHashHex,
		StakerBtcPkHex:            stakerBtcPkHex,
		FinalityProviderBtcPksHex: finalityProviderBtcPksHex,
		StakingAmount:             stakingAmount,
	}
}
