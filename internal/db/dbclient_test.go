//go:build integration

package db_test

import (
	"context"
	"fmt"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"testing"
	"time"
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

	// there can be only 1 container with the same name, so we add
	// random string in the end in case there is still old container running
	containerName := "mongo-integration-tests-db-" + utils.RandomAlphaNum(3)
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
