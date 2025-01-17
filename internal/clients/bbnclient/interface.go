package bbnclient

import (
	"context"
	"time"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
)

//go:generate mockery --name=BbnInterface --output=../../../tests/mocks --outpkg=mocks --filename=mock_bbn_client.go
type BbnInterface interface {
	GetCheckpointParams(ctx context.Context) (*CheckpointParams, error)
	GetAllStakingParams(ctx context.Context) (map[uint32]*StakingParams, error)
	GetLatestBlockNumber(ctx context.Context) (int64, error)
	GetBlock(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlock, error)
	GetBlockResults(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlockResults, error)
	Subscribe(
		subscriber, query string,
		healthCheckInterval time.Duration,
		maxEventWaitInterval time.Duration,
		outCapacity ...int,
	) (out <-chan ctypes.ResultEvent, err error)
	UnsubscribeAll(subscriber string) error
	IsRunning() bool
	Start() error
}
