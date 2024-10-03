package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const DelegationCollection = "delegation"

type DelegationDocument struct {
	ID               primitive.ObjectID `bson:"_id"`
	StakingTxHashHex string             `bson:"staking_tx_hash_hex"`
	State            string             `bson:"state"`
}
