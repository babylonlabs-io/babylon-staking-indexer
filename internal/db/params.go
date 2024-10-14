package db

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) SaveGlobalParams(
	ctx context.Context, param *model.GolablParamDocument,
) error {
	collection := db.client.Database(db.dbName).
		Collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    param.Type,
		"version": param.Version,
	}

	update := bson.M{
		"$setOnInsert": param, // Only insert if the document doesn't exist
	}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("error while upserting global params document: %w with type %s and version %d", err, param.Type, param.Version)
	}
	return nil
}
