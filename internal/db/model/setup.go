package model

import (
	"context"
	"fmt"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	FinalityProviderDetailsCollection = "finality_provider_details"
	BTCDelegationDetailsCollection    = "btc_delegation_details"
	TimeLockCollection                = "timelock"
	GlobalParamsCollection            = "global_params"
	LastProcessedHeightCollection     = "last_processed_height"
	BSNCollection                     = "bsn"
)

type index struct {
	Indexes map[string]int
	Unique  bool
}

var collections = map[string][]index{
	FinalityProviderDetailsCollection: {{Indexes: map[string]int{}}},
	BTCDelegationDetailsCollection: {
		{
			Indexes: map[string]int{
				"staker_btc_pk_hex":                       1,
				"btc_delegation_created_bbn_block.height": -1,
				"_id": 1,
			},
			Unique: false,
		},
	},
	TimeLockCollection: {
		{Indexes: map[string]int{"expire_height": 1}, Unique: false},
	},
	GlobalParamsCollection: {
		{Indexes: map[string]int{"type": 1, "version": 1}, Unique: true},
	},
	LastProcessedHeightCollection: {{Indexes: map[string]int{}}},
}

func Setup(ctx context.Context, cfg *config.DbConfig) error {
	credential := options.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	clientOps := options.Client().ApplyURI(cfg.Address).SetAuth(credential)
	client, err := mongo.Connect(ctx, clientOps)
	if err != nil {
		return err
	}

	// Create a context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) //nolint:mnd
	defer cancel()

	// Access a database and create collections.
	database := client.Database(cfg.DbName)

	// Create collections.
	for collection := range collections {
		createCollection(ctx, database, collection)
	}

	for name, idxs := range collections {
		for _, idx := range idxs {
			createIndex(ctx, database, name, idx)
		}
	}

	log.Ctx(ctx).Info().Msg("Collections and Indexes created successfully.")
	return nil
}

func createCollection(ctx context.Context, database *mongo.Database, collectionName string) {
	log := log.Ctx(ctx)
	// Check if the collection already exists.
	if _, err := database.Collection(collectionName).Indexes().CreateOne(ctx, mongo.IndexModel{}); err != nil {
		log.Debug().Msg(fmt.Sprintf("Collection maybe already exists: %s, skip the rest. info: %s", collectionName, err))
		return
	}

	// Create the collection.
	if err := database.CreateCollection(ctx, collectionName); err != nil {
		log.Error().Err(err).Msg("Failed to create collection: " + collectionName)
		return
	}

	log.Debug().Msg("Collection created successfully: " + collectionName)
}

func createIndex(ctx context.Context, database *mongo.Database, collectionName string, idx index) {
	if len(idx.Indexes) == 0 {
		return
	}
	log := log.Ctx(ctx)

	indexKeys := bson.D{}
	for k, v := range idx.Indexes {
		indexKeys = append(indexKeys, bson.E{Key: k, Value: v})
	}

	index := mongo.IndexModel{
		Keys:    indexKeys,
		Options: options.Index().SetUnique(idx.Unique),
	}

	if _, err := database.Collection(collectionName).Indexes().CreateOne(ctx, index); err != nil {
		log.Debug().Msg(fmt.Sprintf("Failed to create index on collection '%s': %v", collectionName, err))
		return
	}

	log.Debug().Msg("Index created successfully on collection: " + collectionName)
}
