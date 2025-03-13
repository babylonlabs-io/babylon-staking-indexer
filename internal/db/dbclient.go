package db

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
)

type Database struct {
	dbName string
	client *mongo.Client
}

func New(ctx context.Context, cfg config.DbConfig) (*Database, error) {
	credential := options.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	clientOps := options.Client().ApplyURI(cfg.Address).SetAuth(credential)
	client, err := mongo.Connect(ctx, clientOps)
	if err != nil {
		return nil, err
	}

	return &Database{
		dbName: cfg.DbName,
		client: client,
	}, nil
}

func (db *Database) Ping(ctx context.Context) error {
	return db.client.Ping(ctx, nil)
}

func (db *Database) collection(name string) *mongo.Collection {
	return db.client.Database(db.dbName).Collection(name)
}
