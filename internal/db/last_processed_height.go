package db

import (
	"context"

	"errors"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) GetLastProcessedBbnHeight(ctx context.Context) (uint64, error) {
	var result model.LastProcessedHeight
	err := db.collection(model.LastProcessedHeightCollection).
		FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// If no document exists, return 0
			return 0, nil
		}
		return 0, err
	}
	return result.Height, nil
}

func (db *Database) UpdateLastProcessedBbnHeight(ctx context.Context, height uint64) error {
	update := bson.M{"$set": bson.M{"height": height}}
	opts := options.Update().SetUpsert(true)
	_, err := db.collection(model.LastProcessedHeightCollection).UpdateOne(ctx, bson.M{}, update, opts)
	return err
}
