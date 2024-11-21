package bbnclient

import (
	"context"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
)

type BbnInterface interface {
	GetCheckpointParams(ctx context.Context) (*CheckpointParams, error)
	GetAllStakingParams(ctx context.Context) (map[uint32]*StakingParams, error)
	GetLatestBlockNumber(ctx context.Context) (int64, error)
	GetBlock(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlock, error)
	GetBlockResults(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlockResults, error)
	Subscribe(subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error)
	UnsubscribeAll(subscriber string) error
	IsRunning() bool
	Start() error
}
