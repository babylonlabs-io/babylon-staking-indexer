package db

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// CHECKPOINT_PARAMS_VERSION is the version of the checkpoint params
	// the value is hardcoded to 0 as the checkpoint params are not expected to change
	// However, we keep the versioning in place for future compatibility and
	// maintain the same pattern as other global params
	CHECKPOINT_PARAMS_VERSION = 0
	CHECKPOINT_PARAMS_TYPE    = "CHECKPOINT"
	STAKING_PARAMS_TYPE       = "STAKING"
)

func (db *Database) SaveStakingParams(
	ctx context.Context, version uint32, params *bbnclient.StakingParams,
) error {
	collection := db.collection(model.GlobalParamsCollection)

	doc := &model.StakingParamsDocument{
		BaseParamsDocument: model.BaseParamsDocument{
			Type:    STAKING_PARAMS_TYPE,
			Version: version,
		},
		Params: params,
	}

	filter := bson.M{
		"type":    STAKING_PARAMS_TYPE,
		"version": version,
	}
	update := bson.M{"$setOnInsert": doc}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (db *Database) SaveCheckpointParams(
	ctx context.Context, params *bbnclient.CheckpointParams,
) error {
	collection := db.collection(model.GlobalParamsCollection)

	doc := &model.CheckpointParamsDocument{
		BaseParamsDocument: model.BaseParamsDocument{
			Type:    CHECKPOINT_PARAMS_TYPE,
			Version: CHECKPOINT_PARAMS_VERSION, // hardcoded as 0
		},
		Params: params,
	}

	filter := bson.M{
		"type":    CHECKPOINT_PARAMS_TYPE,
		"version": CHECKPOINT_PARAMS_VERSION, // hardcoded as 0
	}
	update := bson.M{"$setOnInsert": doc}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (db *Database) GetStakingParams(ctx context.Context, version uint32) (*bbnclient.StakingParams, error) {
	collection := db.collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    STAKING_PARAMS_TYPE,
		"version": version,
	}

	var params model.StakingParamsDocument
	err := collection.FindOne(ctx, filter).Decode(&params)
	if err != nil {
		return nil, fmt.Errorf("failed to get staking params: %w", err)
	}

	return params.Params, nil
}
