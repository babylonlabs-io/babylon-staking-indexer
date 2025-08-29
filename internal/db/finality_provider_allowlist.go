package db

import (
	"context"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SetFPAllowlisted sets is_allowlisted for a subset of FPs in a BSN
func (db *Database) SetFPAllowlisted(ctx context.Context, bsnID string, btcPks []string, value bool) error {
	if len(btcPks) == 0 {
		return nil
	}

	// normalize and dedup
	set := make(map[string]struct{}, len(btcPks))
	for _, pk := range btcPks {
		l := strings.ToLower(pk)
		set[l] = struct{}{}
	}
	lower := make([]string, 0, len(set))
	for k := range set {
		lower = append(lower, k)
	}

	filter := bson.M{"bsn_id": bsnID, "_id": bson.M{"$in": lower}}
	update := bson.M{"$set": bson.M{"is_allowlisted": value}}
	opts := options.Update().SetCollation(&options.Collation{Locale: "en", Strength: 2})

	_, err := db.collection(model.FinalityProviderDetailsCollection).UpdateMany(ctx, filter, update, opts)
	return err
}

// RecomputeFPAllowlistedForBSN sets all FPs in the BSN to false, then sets true for those in allowlist
func (db *Database) RecomputeFPAllowlistedForBSN(ctx context.Context, bsnID string, allowlist []string) error {
	// set all to false
	filterAll := bson.M{"bsn_id": bsnID}
	updateAll := bson.M{"$set": bson.M{"is_allowlisted": false}}
	_, err := db.collection(model.FinalityProviderDetailsCollection).UpdateMany(ctx, filterAll, updateAll)
	if err != nil {
		return err
	}

	if len(allowlist) == 0 {
		return nil
	}

	return db.SetFPAllowlisted(ctx, bsnID, allowlist, true)
}
