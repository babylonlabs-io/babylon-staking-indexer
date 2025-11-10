package db

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
)

// CalculateActiveStatsAggregated calculates stats using MongoDB aggregation pipeline
// This is much more efficient than loading all delegations into memory
func (db *Database) CalculateActiveStatsAggregated(ctx context.Context) (uint64, uint64, []*FinalityProviderStatsResult, error) {
	collection := db.collection(model.BTCDelegationDetailsCollection)

	// Pipeline to calculate overall stats
	overallPipeline := bson.A{
		// Match only ACTIVE delegations
		bson.M{
			"$match": bson.M{
				"state": "ACTIVE",
			},
		},
		// Group to calculate totals
		bson.M{
			"$group": bson.M{
				"_id":               nil,
				"total_tvl":         bson.M{"$sum": "$staking_amount"},
				"total_delegations": bson.M{"$sum": 1},
			},
		},
	}

	// Execute overall stats aggregation
	cursor, err := collection.Aggregate(ctx, overallPipeline)
	if err != nil {
		return 0, 0, nil, err
	}
	defer cursor.Close(ctx)

	var overallTvl uint64 = 0
	var overallDelegations uint64 = 0

	if cursor.Next(ctx) {
		var result struct {
			TotalTvl         uint64 `bson:"total_tvl"`
			TotalDelegations uint64 `bson:"total_delegations"`
		}
		if err := cursor.Decode(&result); err != nil {
			return 0, 0, nil, err
		}
		overallTvl = result.TotalTvl
		overallDelegations = result.TotalDelegations
	}

	// If no active delegations, return early
	if overallDelegations == 0 {
		return 0, 0, []*FinalityProviderStatsResult{}, nil
	}

	// Pipeline to calculate per-FP stats
	fpPipeline := bson.A{
		// Match only ACTIVE delegations
		bson.M{
			"$match": bson.M{
				"state": "ACTIVE",
			},
		},
		// Unwind the finality_provider_btc_pks_hex array to process each FP separately
		bson.M{
			"$unwind": "$finality_provider_btc_pks_hex",
		},
		// Add a computed field with lowercase FP key for proper grouping
		bson.M{
			"$addFields": bson.M{
				"fp_lowercase": bson.M{"$toLower": "$finality_provider_btc_pks_hex"},
			},
		},
		// Group by lowercase FP public key to handle case-insensitive grouping
		bson.M{
			"$group": bson.M{
				"_id":                "$fp_lowercase",
				"active_tvl":         bson.M{"$sum": "$staking_amount"},
				"active_delegations": bson.M{"$sum": 1},
			},
		},
	}

	// Execute FP stats aggregation
	fpCursor, err := collection.Aggregate(ctx, fpPipeline)
	if err != nil {
		return 0, 0, nil, err
	}
	defer fpCursor.Close(ctx)

	var fpStatsRaw []struct {
		FpBtcPkHex        string `bson:"_id"`
		ActiveTvl         uint64 `bson:"active_tvl"`
		ActiveDelegations uint64 `bson:"active_delegations"`
	}
	if err := fpCursor.All(ctx, &fpStatsRaw); err != nil {
		return 0, 0, nil, err
	}

	// Convert to FinalityProviderStatsResult
	fpStats := make([]*FinalityProviderStatsResult, len(fpStatsRaw))
	for i, raw := range fpStatsRaw {
		fpStats[i] = &FinalityProviderStatsResult{
			FpBtcPkHex:        raw.FpBtcPkHex,
			ActiveTvl:         raw.ActiveTvl,
			ActiveDelegations: raw.ActiveDelegations,
		}
	}

	return overallTvl, overallDelegations, fpStats, nil
}
