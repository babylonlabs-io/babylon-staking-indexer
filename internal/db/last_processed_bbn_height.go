package db

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) GetLastProcessedBBNHeight(ctx context.Context) (uint64, error) {
	var result model.LastProcessedHeight
	err := db.client.Database(db.dbName).
		Collection(model.LastProcessedHeightCollection).
		FindOne(ctx, bson.M{}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		// If no document exists, return 0
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return result.Height, nil
}

func (db *Database) UpdateLastProcessedBBNHeight(ctx context.Context, height uint64) error {
	update := bson.M{"$set": bson.M{"height": height}}
	opts := options.Update().SetUpsert(true)
	_, err := db.client.Database(db.dbName).
		Collection(model.LastProcessedHeightCollection).
		UpdateOne(ctx, bson.M{}, update, opts)
	return err
}
