package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/mongo"
)

func (db *Database) SaveBSN(ctx context.Context, consumer *model.BSN) error {
	_, err := db.collection(model.BSNCollection).
		InsertOne(ctx, consumer)
	if err != nil {
		var writeErr mongo.WriteException
		if errors.As(err, &writeErr) {
			for _, e := range writeErr.WriteErrors {
				if mongo.IsDuplicateKeyError(e) {
					return &DuplicateKeyError{
						Key:     consumer.ID,
						Message: "event consumer already exists",
					}
				}
			}
		}
		return err
	}

	return nil
}

func (db *Database) GetBSNByID(ctx context.Context, id string) (*model.BSN, error) {
	filter := map[string]any{"_id": id}
	res := db.collection(model.BSNCollection).
		FindOne(ctx, filter)

	var consumer model.BSN
	err := res.Decode(&consumer)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     consumer.ID,
				Message: "event consumer not found by id",
			}
		}
		return nil, err
	}

	return &consumer, nil
}
