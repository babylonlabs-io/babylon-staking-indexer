package db

import (
	"context"
	"fmt"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
)

func (db *Database) UpdateDelegationStakerBabylonAddress(ctx context.Context, stakingTxHash, stakerAddr string) error {
	filter := bson.M{"_id": stakingTxHash}
	update := bson.M{
		"$set": bson.M{
			"staker_babylon_address": stakerAddr,
		},
	}
	result, err := db.collection(model.BTCDelegationDetailsCollection).
		UpdateOne(ctx, filter, update)

	fmt.Println("Result", result)

	return err
}
