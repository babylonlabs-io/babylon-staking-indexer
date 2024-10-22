package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/mongo"
)

func (db *Database) SaveNewTimeLockExpire(
	ctx context.Context, stakingTxHashHex string,
	expireHeight uint32, txType string,
) error {
	tlDoc := model.NewTimeLockDocument(stakingTxHashHex, expireHeight, txType)
	_, err := db.client.Database(db.dbName).
		Collection(model.TimeLockCollection).
		InsertOne(ctx, tlDoc)
	if err != nil {
		var writeErr mongo.WriteException
		if errors.As(err, &writeErr) {
			for _, e := range writeErr.WriteErrors {
				if mongo.IsDuplicateKeyError(e) {
					return &DuplicateKeyError{
						Key:     tlDoc.StakingTxHashHex,
						Message: "timelock already exists",
					}
				}
			}
		}
		return err
	}
	return nil
}
