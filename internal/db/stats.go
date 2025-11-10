package db

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UpsertOverallStats updates or inserts overall stats
func (db *Database) UpsertOverallStats(
	ctx context.Context,
	activeTvl uint64,
	activeDelegations uint64,
) error {
	filter := bson.M{"_id": "overall_stats"}
	update := bson.M{
		"$set": bson.M{
			"active_tvl":         activeTvl,
			"active_delegations": activeDelegations,
			"last_updated":       time.Now().Unix(),
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err := db.collection(model.OverallStatsCollection).UpdateOne(ctx, filter, update, opts)
	return err
}

// UpsertFinalityProviderStats updates or inserts finality provider stats in separate collection
func (db *Database) UpsertFinalityProviderStats(
	ctx context.Context,
	fpBtcPkHex string,
	activeTvl uint64,
	activeDelegations uint64,
) error {
	filter := bson.M{"_id": fpBtcPkHex}
	update := bson.M{
		"$set": bson.M{
			"active_tvl":         activeTvl,
			"active_delegations": activeDelegations,
			"last_updated":       time.Now().Unix(),
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err := db.collection(model.FinalityProviderStatsCollection).UpdateOne(ctx, filter, update, opts)
	return err
}
