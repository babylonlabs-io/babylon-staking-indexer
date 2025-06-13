package model

import btcstkconsumer "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"

type BSNType string

const (
	BSNTypeCosmos = "cosmos"
	BSNTypeRollup = "rollup"
)

type BSN struct {
	ID                string         `bson:"_id"`
	Name              string         `bson:"name"`
	Description       string         `bson:"description"`
	MaxMultiStakedFPS uint32         `bson:"max_multi_staked_fps"` // max number of finality providers from consumer
	Type              BSNType        `bson:"type"`
	RollupMetadata    *ETHL2Metadata `bson:"rollup_metadata"`
}

type ETHL2Metadata struct {
	FinalityContractAddress string `bson:"finality_contract_address"`
}

func FromEventConsumerRegistered(event *btcstkconsumer.EventConsumerRegistered) *BSN {
	var consumerType BSNType
	switch event.ConsumerType {
	case btcstkconsumer.ConsumerType_COSMOS:
		consumerType = BSNTypeCosmos
	case btcstkconsumer.ConsumerType_ROLLUP:
		consumerType = BSNTypeRollup
	}

	var rollupMetadata *ETHL2Metadata
	if event.RollupConsumerMetadata != nil {
		rollupMetadata = &ETHL2Metadata{
			FinalityContractAddress: event.RollupConsumerMetadata.FinalityContractAddress,
		}
	}

	return &BSN{
		ID:                event.ConsumerId,
		Name:              event.ConsumerName,
		Description:       event.ConsumerDescription,
		MaxMultiStakedFPS: event.MaxMultiStakedFps,
		Type:              consumerType,
		RollupMetadata:    rollupMetadata,
	}
}
