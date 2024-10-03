package db

import (
	"context"
)

type DbInterface interface {
	Ping(ctx context.Context) error
}
