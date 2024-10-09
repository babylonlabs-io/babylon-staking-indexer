package model

import (
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const DelegationCollection = "delegation"

type DelegationDocument struct {
	ID               primitive.ObjectID    `bson:"_id"`
	StakingTxHashHex string                `bson:"staking_tx_hash_hex"`
	State            types.DelegationState `bson:"state"`
	// TODO: Placeholder for more fields
}
