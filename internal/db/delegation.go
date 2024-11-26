package db

import (
	"context"
	"errors"
	"fmt"
	"log"

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
	ctx context.Context,
	stakingTxHash string,
	qualifiedPreviousStates []types.DelegationState,
	newState types.DelegationState,
	newSubState *types.DelegationSubState,
) error {
	qualifiedStateStrs := make([]string, len(qualifiedPreviousStates))
	for i, state := range qualifiedPreviousStates {
		qualifiedStateStrs[i] = state.String()
	}

	filter := bson.M{
		"_id":   stakingTxHash,
		"state": bson.M{"$in": qualifiedStateStrs},
	}

	updateFields := bson.M{
		"state": newState.String(),
	}

	if newSubState != nil {
		updateFields["sub_state"] = newSubState.String()
	}

	update := bson.M{
		"$set": updateFields,
	}

	res := db.client.Database(db.dbName).
		Collection(model.BTCDelegationDetailsCollection).
		FindOneAndUpdate(ctx, filter, update)

	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found or current state is not qualified states",
			}
		}
		return res.Err()
	}

	return nil
}

func (db *Database) GetBTCDelegationState(
	ctx context.Context, stakingTxHash string,
) (*types.DelegationState, error) {
	delegation, err := db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if err != nil {
		return nil, err
	}
	return &delegation.State, nil
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
	if details.StartHeight != 0 {
		updateFields["start_height"] = details.StartHeight
	}
	if details.EndHeight != 0 {
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

func (db *Database) SaveBTCDelegationUnbondingCovenantSignature(
	ctx context.Context, stakingTxHash string, covenantBtcPkHex string, signatureHex string,
) error {
	filter := bson.M{"_id": stakingTxHash}
	update := bson.M{
		"$push": bson.M{
			"covenant_unbonding_signatures": bson.M{
				"covenant_btc_pk_hex": covenantBtcPkHex,
				"signature_hex":       signatureHex,
			},
		},
	}
	_, err := db.client.Database(db.dbName).
		Collection(model.BTCDelegationDetailsCollection).
		UpdateOne(ctx, filter, update)

	return err
}

func (db *Database) GetBTCDelegationByStakingTxHash(
	ctx context.Context, stakingTxHash string,
) (*model.BTCDelegationDetails, error) {
	filter := bson.M{"_id": stakingTxHash}

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

func (db *Database) UpdateDelegationsStateByFinalityProvider(
	ctx context.Context,
	fpBTCPKHex string,
	newState types.DelegationState,
) error {
	filter := bson.M{
		"finality_provider_btc_pks_hex": fpBTCPKHex,
	}

	update := bson.M{
		"$set": bson.M{
			"state": newState.String(),
		},
	}

	result, err := db.client.Database(db.dbName).
		Collection(model.BTCDelegationDetailsCollection).
		UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update delegations: %w", err)
	}

	log.Printf("Updated %d delegations for finality provider %s to state %s",
		result.ModifiedCount,
		fpBTCPKHex,
		newState.String(),
	)
	return nil
}
