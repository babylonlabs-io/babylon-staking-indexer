package bbnclient

import (
	"context"
	"time"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
)

//go:generate mockery --name=BbnInterface --output=../../../tests/mocks --outpkg=mocks --filename=mock_bbn_client.go
type BbnInterface interface {
	GetCheckpointParams(ctx context.Context) (*CheckpointParams, error)
	// GetStakingParams returns all staking parameters starting from the given version (inclusive)
	GetStakingParams(ctx context.Context, minVersion uint32) (map[uint32]*StakingParams, error)
	GetFinalityParams(ctx context.Context) (*FinalityParams, error)
	GetLatestBlockNumber(ctx context.Context) (int64, error)
	GetChainID(ctx context.Context) (string, error)
	GetBlock(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlock, error)
	GetBlockResults(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlockResults, error)
	BabylonStakerAddress(ctx context.Context, stakingTxHashHex string) (string, error)
	Subscribe(
		ctx context.Context,
		subscriber, query string,
		healthCheckInterval time.Duration,
		maxEventWaitInterval time.Duration,
		outCapacity ...int,
	) (out <-chan ctypes.ResultEvent, err error)
	UnsubscribeAll(ctx context.Context, subscriber string) error
	IsRunning() bool
	Start() error
}
