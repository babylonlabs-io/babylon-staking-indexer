package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) SaveBSN(ctx context.Context, bsn *model.BSN) error {
	filter := bson.M{"_id": bsn.ID}
	update := bson.M{"$set": bsn}
	opts := options.Update().SetUpsert(true)

	_, err := db.collection(model.BSNCollection).
		UpdateOne(ctx, filter, update, opts)
	return err
}

func (db *Database) GetBSNByID(ctx context.Context, id string) (*model.BSN, error) {
	filter := map[string]any{"_id": id}
	res := db.collection(model.BSNCollection).
		FindOne(ctx, filter)

	var bsn model.BSN
	err := res.Decode(&bsn)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     bsn.ID,
				Message: "bsn not found by id",
			}
		}
		return nil, err
	}

	return &bsn, nil
}
