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

	_, err := db.collection(model.StatsCollection).UpdateOne(ctx, filter, update, opts)
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

// ZeroOutInactiveFinalityProviderStats resets active stats to zero for any finality
// provider that has a stored stats document but is no longer present in the current
// active aggregation. activeFpBtcPkHex must contain the FP keys from the latest active
// aggregation. Only currently non-zero documents are touched to avoid rewriting rows
// that are already zeroed.
func (db *Database) ZeroOutInactiveFinalityProviderStats(
	ctx context.Context,
	activeFpBtcPkHex []string,
) error {
	filter := bson.M{
		"_id": bson.M{"$nin": activeFpBtcPkHex},
		"$or": bson.A{
			bson.M{"active_tvl": bson.M{"$gt": 0}},
			bson.M{"active_delegations": bson.M{"$gt": 0}},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"active_tvl":         0,
			"active_delegations": 0,
			"last_updated":       time.Now().Unix(),
		},
	}

	_, err := db.collection(model.FinalityProviderStatsCollection).UpdateMany(ctx, filter, update)
	return err
}
