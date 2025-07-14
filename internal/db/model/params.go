package model

import "github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"

// Base document for common fields
type BaseParamsDocument struct {
	Type    string `bson:"type"`
	Version uint32 `bson:"version"`
}

// Specific document for staking params
type StakingParamsDocument struct {
	BaseParamsDocument `bson:",inline"`
	Params             *bbnclient.StakingParams `bson:"params"`
}

// Specific document for checkpoint params
type CheckpointParamsDocument struct {
	BaseParamsDocument `bson:",inline"`
	Params             *bbnclient.CheckpointParams `bson:"params"`
}
