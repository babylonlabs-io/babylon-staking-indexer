package model

import btcstkconsumer "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"

type EventConsumerType string

const (
	EventConsumerTypeCosmos = "cosmos"
	EventConsumerTypeRollup = "rollup"
)

type BSN struct {
	ID                     string                 `bson:"_id"`
	Name                   string                 `bson:"name"`
	Description            string                 `bson:"description"`
	MaxMultiStakedFPS      uint32                 `bson:"max_multi_staked_fps"` // max number of finality providers from consumer
	Type                   EventConsumerType      `bson:"type"`
	RollupConsumerMetadata *ETHL2ConsumerMetadata `bson:"rollup_consumer_metadata"`
}

type ETHL2ConsumerMetadata struct {
	FinalityContractAddress string `bson:"finality_contract_address"`
}

func FromEventConsumerRegistered(event *btcstkconsumer.EventConsumerRegistered) *BSN {
	var consumerType EventConsumerType
	switch event.ConsumerType {
	case btcstkconsumer.ConsumerType_COSMOS:
		consumerType = EventConsumerTypeCosmos
	case btcstkconsumer.ConsumerType_ROLLUP:
		consumerType = EventConsumerTypeRollup
	}

	var rollupMetadata *ETHL2ConsumerMetadata
	if event.RollupConsumerMetadata != nil {
		rollupMetadata = &ETHL2ConsumerMetadata{
			FinalityContractAddress: event.RollupConsumerMetadata.FinalityContractAddress,
		}
	}

	return &BSN{
		ID:                     event.ConsumerId,
		Name:                   event.ConsumerName,
		Description:            event.ConsumerDescription,
		MaxMultiStakedFPS:      event.MaxMultiStakedFps,
		Type:                   consumerType,
		RollupConsumerMetadata: rollupMetadata,
	}
}
