package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) SaveBSN(ctx context.Context, bsn *model.BSN) error {
	_, err := db.collection(model.BSNCollection).
		InsertOne(ctx, bsn)
	if err != nil {
		var writeErr mongo.WriteException
		if errors.As(err, &writeErr) {
			for _, e := range writeErr.WriteErrors {
				if mongo.IsDuplicateKeyError(e) {
					return &DuplicateKeyError{
						Key:     bsn.ID,
						Message: "bsn already exists",
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

// GetBSNByAddress retrieves a BSN by its finality contract address
func (db *Database) GetBSNByAddress(ctx context.Context, address string) (*model.BSN, error) {
	filter := bson.M{"rollup_metadata.finality_contract_address": address}
	res := db.collection(model.BSNCollection).FindOne(ctx, filter)

	var bsn model.BSN
	err := res.Decode(&bsn)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     address,
				Message: "bsn not found by address",
			}
		}
		return nil, err
	}

	return &bsn, nil
}

// UpdateBSNAllowlist updates the BSN allowlist with the provided allowlist
func (db *Database) UpdateBSNAllowlist(ctx context.Context, address string, allowlist []string) error {
	log := log.Ctx(ctx)

	filter := bson.M{"rollup_metadata.finality_contract_address": address}
	update := bson.M{
		"$set": bson.M{
			"rollup_metadata.allowlist": allowlist,
		},
	}

	result, err := db.collection(model.BSNCollection).UpdateOne(ctx, filter, update, options.Update().SetUpsert(false))
	if err != nil {
		return fmt.Errorf("failed to update BSN allowlist: %w", err)
	}

	if result.MatchedCount == 0 {
		log.Warn().
			Str("address", address).
			Msg("No BSN found to update allowlist for")
		return nil
	}

	log.Info().
		Str("address", address).
		Int("allowlist_size", len(allowlist)).
		Msg("BSN allowlist updated successfully")

	return nil
}
