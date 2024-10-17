package db

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/rs/zerolog/log"
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
	collection := db.client.Database(db.dbName).
		Collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    STAKING_PARAMS_TYPE,
		"version": version,
	}

	update := bson.M{
		"$setOnInsert": &model.GlobalParamsDocument{
			Type:    STAKING_PARAMS_TYPE,
			Version: version,
			Params:  params,
		},
	}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("failed to save staking params: %w", err)
	}
	return nil
}

func (db *Database) SaveCheckpointParams(
	ctx context.Context, params *bbnclient.CheckpointParams,
) error {
	log.Debug().Msg("Saving checkpoint params")

	collection := db.client.Database(db.dbName).
		Collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    CHECKPOINT_PARAMS_TYPE,
		"version": CHECKPOINT_PARAMS_VERSION,
	}

	update := bson.M{
		"$setOnInsert": &model.GlobalParamsDocument{
			Type:    CHECKPOINT_PARAMS_TYPE,
			Version: CHECKPOINT_PARAMS_VERSION,
			Params:  params,
		},
	}

	log.Debug().
		Interface("filter", filter).
		Interface("update", update).
		Msg("Attempting to update checkpoint params")

	result, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		log.Error().Err(err).Msg("Failed to save checkpoint params")
		return fmt.Errorf("failed to save checkpoint params: %w", err)
	}

	log.Info().
		Int64("matched_count", result.MatchedCount).
		Int64("modified_count", result.ModifiedCount).
		Int64("upserted_count", result.UpsertedCount).
		Msg("Successfully saved checkpoint params")

	return nil
}
