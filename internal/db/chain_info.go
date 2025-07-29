package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const networkInfoID = "singleton"

type networkInfoDoc struct {
	ID                 string `bson:"_id"`
	*model.NetworkInfo `bson:",inline"`
}

func (db *Database) GetNetworkInfo(ctx context.Context) (*model.NetworkInfo, error) {
	filter := map[string]any{"_id": networkInfoID}
	res := db.collection(model.NetworkInfoCollection).FindOne(ctx, filter)

	var doc networkInfoDoc
	err := res.Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     networkInfoID,
				Message: "network info not found",
			}
		}
		return nil, err
	}

	return doc.NetworkInfo, nil
}

func (db *Database) UpsertNetworkInfo(ctx context.Context, networkInfo *model.NetworkInfo) error {
	collection := db.collection(model.NetworkInfoCollection)

	doc := networkInfoDoc{
		ID:          networkInfoID,
		NetworkInfo: networkInfo,
	}

	filter := bson.M{
		"_id": networkInfoID,
	}
	update := bson.M{"$set": doc}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
