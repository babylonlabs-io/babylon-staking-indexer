package db

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// checkpointParamsVersion is the version of the checkpoint params
	// the value is hardcoded to 0 as the checkpoint params are not expected to change
	// However, we keep the versioning in place for future compatibility and
	// maintain the same pattern as other global params
	checkpointParamsVersion = 0
	fpParamsVersion         = 0
	fpParamsType            = "FP"
	checkpointParamsType    = "CHECKPOINT"
	stakingParamsType       = "STAKING"
)

func (db *Database) SaveStakingParams(
	ctx context.Context, version uint32, params *bbnclient.StakingParams,
) error {
	collection := db.collection(model.GlobalParamsCollection)

	doc := &model.StakingParamsDocument{
		BaseParamsDocument: model.BaseParamsDocument{
			Type:    stakingParamsType,
			Version: version,
		},
		Params: params,
	}

	_, err := collection.InsertOne(ctx, doc)
	// nil check is inside IsDuplicateKeyError
	if mongo.IsDuplicateKeyError(err) {
		return &DuplicateKeyError{
			Message: err.Error(),
		}
	}
	return err
}

func (db *Database) SaveCheckpointParams(
	ctx context.Context, params *bbnclient.CheckpointParams,
) error {
	collection := db.collection(model.GlobalParamsCollection)

	doc := &model.CheckpointParamsDocument{
		BaseParamsDocument: model.BaseParamsDocument{
			Type:    checkpointParamsType,
			Version: checkpointParamsVersion, // hardcoded as 0
		},
		Params: params,
	}

	filter := bson.M{
		"type":    checkpointParamsType,
		"version": checkpointParamsVersion, // hardcoded as 0
	}
	update := bson.M{"$set": doc}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (db *Database) SaveFinalityProviderParams(
	ctx context.Context, maxActiveFinalityProviders uint32,
) error {
	collection := db.collection(model.GlobalParamsCollection)

	doc := &model.FPDocument{
		BaseParamsDocument: model.BaseParamsDocument{
			Type:    fpParamsType,
			Version: fpParamsVersion, // hardcoded as 0
		},
		MaxActiveFinalityProviders: maxActiveFinalityProviders,
	}

	filter := bson.M{
		"type":    fpParamsType,
		"version": fpParamsVersion, // hardcoded as 0
	}
	update := bson.M{"$set": doc}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (db *Database) GetCheckpointParams(ctx context.Context) (*bbnclient.CheckpointParams, error) {
	collection := db.collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    checkpointParamsType,
		"version": checkpointParamsVersion,
	}

	var params model.CheckpointParamsDocument
	err := collection.FindOne(ctx, filter).Decode(&params)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint params: %w", err)
	}

	return params.Params, nil
}

func (db *Database) GetStakingParams(ctx context.Context, version uint32) (*bbnclient.StakingParams, error) {
	collection := db.collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    stakingParamsType,
		"version": version,
	}

	var params model.StakingParamsDocument
	err := collection.FindOne(ctx, filter).Decode(&params)
	if err != nil {
		return nil, fmt.Errorf("failed to get staking params: %w", err)
	}

	return params.Params, nil
}
