//go:build integration

package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/testutil"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoUsername     = "user"
	mongoPassword     = "password"
	mongoDatabaseName = "test-database"

	// this version corresponds to docker tag for mongodb
	// it should be in sync with mongo version used in production
	mongoVersion = "7.0.5"
)

var testDB *db.Database

// mongo connected to test database, used for truncating collections
var mongoDB *mongo.Database

func TestMain(m *testing.M) {
	// first setup container with MongoDb
	dbConfig, cleanup, err := setupMongoContainer()
	if err != nil {
		log.Fatalf("failed to setup mongo container: %v", err)
	}

	// apply migrations
	err = model.Setup(context.Background(), dbConfig)
	if err != nil {
		cleanup()
		log.Fatalf("failed to init mongo database: %v", err)
	}

	// using config from container mongo initialize client used in tests
	testDB, err = setupClient(dbConfig)
	if err != nil {
		cleanup()
		log.Fatalf("failed to setup client: %v", err)
	}

	// setup mongo client used for preparing/cleaning data
	mongoDB, err = setupMongoClient(dbConfig)
	if err != nil {
		cleanup()
		log.Fatalf("failed to setup mongo client: %v", err)
	}

	// integration tests run on this line
	code := m.Run()
	cleanup()

	os.Exit(code)
}

// setupMongoContainer setups container with mongodb returning db credentials through config.DbConfig, cleanup function
// and an error if any. Cleanup function MUST be called in the end to cleanup docker resources
func setupMongoContainer() (*config.DbConfig, func(), error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, err
	}

	// generate random string for container name
	randomString, err := testutil.RandomAlphaNum(3)
	if err != nil {
		return nil, nil, err
	}

	// there can be only 1 container with the same name, so we add
	// random string in the end in case there is still old container running
	containerName := "mongo-integration-tests-db-" + randomString
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       containerName,
		Repository: "mongo",
		Tag:        mongoVersion,
		Env: []string{
			"MONGO_INITDB_ROOT_USERNAME=" + mongoUsername,
			"MONGO_INITDB_ROOT_PASSWORD=" + mongoPassword,
			"MONGO_INITDB_DATABASE=" + mongoDatabaseName,
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		err := pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge resource: %v", err)
		}
	}

	// get host port (randomly chosen) that is mapped to mongo port inside container
	hostPort := resource.GetPort("27017/tcp")

	return &config.DbConfig{
		Username: mongoUsername,
		Password: mongoPassword,
		DbName:   mongoDatabaseName,
		Address:  fmt.Sprintf("mongodb://localhost:%s/", hostPort),
	}, cleanup, nil
}

func resetDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collections := []string{
		model.FinalityProviderDetailsCollection,
		model.BTCDelegationDetailsCollection,
		model.TimeLockCollection,
		model.GlobalParamsCollection,
		model.LastProcessedHeightCollection,
	}

	for _, collection := range collections {
		_, err := mongoDB.Collection(collection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)
	}
}

func setupClient(cfg *config.DbConfig) (*db.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.New(ctx, *cfg)
}

func setupMongoClient(cfg *config.DbConfig) (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	credential := options.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	clientOps := options.Client().ApplyURI(cfg.Address).SetAuth(credential)
	client, err := mongo.Connect(ctx, clientOps)
	if err != nil {
		return nil, err
	}

	return client.Database(cfg.DbName), nil
}

func TestUpdateBabylonFinalityProviderBsnId(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	// Create a service instance for testing
	service := &Service{
		db: testDB,
	}

	// Setup network info
	networkInfo := &model.NetworkInfo{
		ChainID: "test-chain-id",
	}
	err := testDB.UpsertNetworkInfo(ctx, networkInfo)
	require.NoError(t, err)

	t.Run("successful update of FPs with missing BSN IDs", func(t *testing.T) {
		// Create test finality providers - some with BSN IDs, some without
		fp1 := &model.FinalityProviderDetails{
			BtcPk: "btc_pk_1",
			BsnID: "", // Missing BSN ID
		}
		fp2 := &model.FinalityProviderDetails{
			BtcPk: "btc_pk_2",
			BsnID: "existing-bsn-id", // Has BSN ID
		}
		fp3 := &model.FinalityProviderDetails{
			BtcPk: "btc_pk_3",
			BsnID: "", // Missing BSN ID
		}

		// Save finality providers
		err := testDB.SaveNewFinalityProvider(ctx, fp1)
		require.NoError(t, err)
		err = testDB.SaveNewFinalityProvider(ctx, fp2)
		require.NoError(t, err)
		err = testDB.SaveNewFinalityProvider(ctx, fp3)
		require.NoError(t, err)

		// Call the method
		updatedCount, err := service.UpdateBabylonFinalityProviderBsnId(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(2), updatedCount) // Should update 2 FPs

		// Verify the updates
		updatedFp1, err := testDB.GetFinalityProviderByBtcPk(ctx, fp1.BtcPk)
		require.NoError(t, err)
		require.Equal(t, networkInfo.ChainID, updatedFp1.BsnID)

		updatedFp2, err := testDB.GetFinalityProviderByBtcPk(ctx, fp2.BtcPk)
		require.NoError(t, err)
		require.Equal(t, "existing-bsn-id", updatedFp2.BsnID) // Should remain unchanged

		updatedFp3, err := testDB.GetFinalityProviderByBtcPk(ctx, fp3.BtcPk)
		require.NoError(t, err)
		require.Equal(t, networkInfo.ChainID, updatedFp3.BsnID)
	})

	t.Run("no updates needed - all FPs have BSN IDs", func(t *testing.T) {
		// Create finality providers with BSN IDs
		fp1 := &model.FinalityProviderDetails{
			BtcPk: "btc_pk_4",
			BsnID: "existing-bsn-id-1",
		}
		fp2 := &model.FinalityProviderDetails{
			BtcPk: "btc_pk_5",
			BsnID: "existing-bsn-id-2",
		}

		// Save finality providers
		err := testDB.SaveNewFinalityProvider(ctx, fp1)
		require.NoError(t, err)
		err = testDB.SaveNewFinalityProvider(ctx, fp2)
		require.NoError(t, err)

		// Call the method
		updatedCount, err := service.UpdateBabylonFinalityProviderBsnId(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), updatedCount) // Should update 0 FPs

		// Verify no changes
		updatedFp1, err := testDB.GetFinalityProviderByBtcPk(ctx, fp1.BtcPk)
		require.NoError(t, err)
		require.Equal(t, "existing-bsn-id-1", updatedFp1.BsnID)

		updatedFp2, err := testDB.GetFinalityProviderByBtcPk(ctx, fp2.BtcPk)
		require.NoError(t, err)
		require.Equal(t, "existing-bsn-id-2", updatedFp2.BsnID)
	})

	t.Run("no finality providers exist", func(t *testing.T) {
		// Ensure no finality providers exist
		finalityProviders, err := testDB.GetAllFinalityProviders(ctx)
		require.NoError(t, err)
		require.Empty(t, finalityProviders)

		// Call the method
		updatedCount, err := service.UpdateBabylonFinalityProviderBsnId(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), updatedCount) // Should update 0 FPs
	})

	t.Run("network info not found", func(t *testing.T) {
		// Remove network info
		_, err := mongoDB.Collection(model.NetworkInfoCollection).DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Create a finality provider with missing BSN ID
		fp := &model.FinalityProviderDetails{
			BtcPk: "btc_pk_6",
			BsnID: "", // Missing BSN ID
		}
		err = testDB.SaveNewFinalityProvider(ctx, fp)
		require.NoError(t, err)

		// Call the method - should fail
		_, err = service.UpdateBabylonFinalityProviderBsnId(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get network info")

		// Restore network info for other tests
		err = testDB.UpsertNetworkInfo(ctx, networkInfo)
		require.NoError(t, err)
	})
}
