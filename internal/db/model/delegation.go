package model

import (
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
)

type DelegationDocument struct {
	StakingTxHashHex string                `bson:"_id"` // Primary key
	State            types.DelegationState `bson:"state"`
	// TODO: Placeholder for more fields
}
