package btcclient

import (
	"time"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"context"
)

type btcClientWithMetrics struct {
	btc BtcInterface
}

func NewBTCClientWithMetrics(btc BtcInterface) *btcClientWithMetrics {
	return &btcClientWithMetrics{btc: btc}
}

func (b *btcClientWithMetrics) GetTipHeight(ctx context.Context) (uint64, error) {
	return runBtcClientMethodWithMetrics("GetTipHeight", func() (uint64, error) {
		return b.btc.GetTipHeight(ctx)
	})
}

func (b *btcClientWithMetrics) GetBlockTimestamp(ctx context.Context, height uint32) (int64, error) {
	return runBtcClientMethodWithMetrics("GetBlockTimestamp", func() (int64, error) {
		return b.btc.GetBlockTimestamp(ctx, height)
	})
}

func runBtcClientMethodWithMetrics[T any](method string, f func() (T, error)) (T, error) {
	startTime := time.Now()
	v, err := f()
	duration := time.Since(startTime)

	metrics.RecordBTCClientLatency(duration, method, err != nil)
	return v, err
}
