package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (db *Database) SaveNewBTCDelegation(
	ctx context.Context, delegationDoc *model.BTCDelegationDetails,
) error {
	_, err := db.client.Database(db.dbName).
		Collection(model.BTCDelegationDetailsCollection).
		InsertOne(ctx, delegationDoc)
	if err != nil {
		var writeErr mongo.WriteException
		if errors.As(err, &writeErr) {
			for _, e := range writeErr.WriteErrors {
				if mongo.IsDuplicateKeyError(e) {
					return &DuplicateKeyError{
						Key:     delegationDoc.StakingTxHashHex,
						Message: "delegation already exists",
					}
				}
			}
		}
		return err
	}
	return nil
}

func (db *Database) UpdateBTCDelegationState(
	ctx context.Context, stakingTxHash string, newState types.DelegationState,
) error {
	filter := map[string]interface{}{"_id": stakingTxHash}
	update := map[string]interface{}{"$set": map[string]string{"state": newState.String()}}

	res := db.client.Database(db.dbName).
		Collection(model.BTCDelegationDetailsCollection).
		FindOneAndUpdate(ctx, filter, update)

	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found when updating state",
			}
		}
		return res.Err()
	}

	return nil
}

func (db *Database) UpdateBTCDelegationDetails(
	ctx context.Context,
	stakingTxHash string,
	details *model.BTCDelegationDetails,
) error {
	updateFields := bson.M{}

	// Only add fields to updateFields if they are not empty
	if details.State.String() != "" {
		updateFields["state"] = details.State.String()
	}
	if details.StartHeight != "" {
		updateFields["start_height"] = details.StartHeight
	}
	if details.EndHeight != "" {
		updateFields["end_height"] = details.EndHeight
	}

	// Perform the update only if there are fields to update
	if len(updateFields) > 0 {
		filter := bson.M{"_id": stakingTxHash}
		update := bson.M{"$set": updateFields}

		res, err := db.client.Database(db.dbName).
			Collection(model.BTCDelegationDetailsCollection).
			UpdateOne(ctx, filter, update)

		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			return &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found when updating details",
			}
		}
	}

	return nil
}

func (db *Database) GetBTCDelegationByStakingTxHash(
	ctx context.Context, stakingTxHash string,
) (*model.BTCDelegationDetails, error) {
	filter := map[string]interface{}{"_id": stakingTxHash}
	res := db.client.Database(db.dbName).
		Collection(model.BTCDelegationDetailsCollection).
		FindOne(ctx, filter)

	var delegationDoc model.BTCDelegationDetails
	err := res.Decode(&delegationDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found when getting by staking tx hash",
			}
		}
		return nil, err
	}

	return &delegationDoc, nil
}
