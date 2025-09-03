package bbnclient

import (
	"context"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
)

type bbnClientWithMetrics struct {
	bbn BbnInterface
}

func NewBBNClientWithMetrics(bbn BbnInterface) *bbnClientWithMetrics {
	return &bbnClientWithMetrics{bbn: bbn}
}

func (b *bbnClientWithMetrics) GetCheckpointParams(ctx context.Context) (*CheckpointParams, error) {
	return runBbnClientMethodWithMetrics("GetCheckpointParams", func() (*CheckpointParams, error) {
		return b.bbn.GetCheckpointParams(ctx)
	})
}

func (b *bbnClientWithMetrics) GetAllStakingParams(ctx context.Context) (map[uint32]*StakingParams, error) {
	return runBbnClientMethodWithMetrics("GetAllStakingParams", func() (map[uint32]*StakingParams, error) {
		return b.bbn.GetAllStakingParams(ctx)
	})
}

func (b *bbnClientWithMetrics) GetStakingParams(ctx context.Context, minVersion uint32) (map[uint32]*StakingParams, error) {
	return runBbnClientMethodWithMetrics("GetStakingParams", func() (map[uint32]*StakingParams, error) {
		return b.bbn.GetStakingParams(ctx, minVersion)
	})
}

func (b *bbnClientWithMetrics) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	return runBbnClientMethodWithMetrics("GetLatestBlockNumber", func() (int64, error) {
		return b.bbn.GetLatestBlockNumber(ctx)
	})
}

func (b *bbnClientWithMetrics) GetChainID(ctx context.Context) (string, error) {
	return runBbnClientMethodWithMetrics("GetChainID", func() (string, error) {
		return b.bbn.GetChainID(ctx)
	})
}

func (b *bbnClientWithMetrics) GetBlock(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlock, error) {
	return runBbnClientMethodWithMetrics("GetBlock", func() (*ctypes.ResultBlock, error) {
		return b.bbn.GetBlock(ctx, blockHeight)
	})
}

func (b *bbnClientWithMetrics) GetBlockResults(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlockResults, error) {
	return runBbnClientMethodWithMetrics("GetBlockResults", func() (*ctypes.ResultBlockResults, error) {
		return b.bbn.GetBlockResults(ctx, blockHeight)
	})
}

func (b *bbnClientWithMetrics) Subscribe(ctx context.Context, subscriber, query string, healthCheckInterval time.Duration, maxEventWaitInterval time.Duration, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	return runBbnClientMethodWithMetrics("Subscribe", func() (<-chan ctypes.ResultEvent, error) {
		return b.bbn.Subscribe(ctx, subscriber, query, healthCheckInterval, maxEventWaitInterval, outCapacity...)
	})
}

func (b *bbnClientWithMetrics) UnsubscribeAll(ctx context.Context, subscriber string) error {
	// this is just auxiliary type in order to call runBbnClientMethodWithMetrics which always returns 2 values
	type zero struct{}
	_, err := runBbnClientMethodWithMetrics[zero]("UnsubscribeAll", func() (zero, error) {
		return zero{}, b.bbn.UnsubscribeAll(ctx, subscriber)
	})

	return err
}

func (b *bbnClientWithMetrics) BabylonStakerAddress(ctx context.Context, stakingTxHashHex string) (string, error) {
	// we don't need to measure latency for this method (it's used only in FillStakerAddr script)
	return b.bbn.BabylonStakerAddress(ctx, stakingTxHashHex)
}

func (b *bbnClientWithMetrics) GetWasmAllowlist(ctx context.Context, contractAddress string) ([]string, error) {
	// we don't need to measure latency for this method (one-off backfill use)
	return b.bbn.GetWasmAllowlist(ctx, contractAddress)
}

func (b *bbnClientWithMetrics) IsRunning() bool {
	return b.bbn.IsRunning()
}

func (b *bbnClientWithMetrics) Start() error {
	return b.bbn.Start()
}

func runBbnClientMethodWithMetrics[T any](method string, f func() (T, error)) (T, error) {
	startTime := time.Now()
	v, err := f()
	duration := time.Since(startTime)

	metrics.RecordBBNClientLatency(duration, method, err != nil)
	return v, err
}
