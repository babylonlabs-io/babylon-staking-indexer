package db

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) SaveNewTimeLockExpire(
	ctx context.Context,
	stakingTxHashHex string,
	expireHeight uint32,
	subState types.DelegationSubState,
) error {
	tlDoc := model.NewTimeLockDocument(stakingTxHashHex, expireHeight, subState)
	_, err := db.collection(model.TimeLockCollection).InsertOne(ctx, tlDoc)
	return err
}

// todo change type from uint64 to something else cause bson.M{"$lte": math.MaxUint64} fails during marshaling
func (db *Database) FindExpiredDelegations(ctx context.Context, btcTipHeight, limit uint64) ([]model.TimeLockDocument, error) {
	client := db.collection(model.TimeLockCollection)
	filter := bson.M{"expire_height": bson.M{"$lte": btcTipHeight}}

	opts := options.Find().SetLimit(int64(limit))
	cursor, err := client.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var delegations []model.TimeLockDocument
	if err = cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	return delegations, nil
}

func (db *Database) DeleteExpiredDelegation(ctx context.Context, stakingTxHashHex string) error {
	client := db.collection(model.TimeLockCollection)
	filter := bson.M{"staking_tx_hash_hex": stakingTxHashHex}

	result, err := client.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete expired delegation with stakingTxHashHex %v: %w", stakingTxHashHex, err)
	}

	// Check if any document was deleted
	if result.DeletedCount == 0 {
		return fmt.Errorf("no expired delegation found with stakingTxHashHex %v", stakingTxHashHex)
	}

	return nil
}
