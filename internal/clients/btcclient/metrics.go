package btcclient

import (
	"time"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
)

type btcClientWithMetrics struct {
	btc BtcInterface
}

func NewBTCClientWithMetrics(btc BtcInterface) *btcClientWithMetrics {
	return &btcClientWithMetrics{btc: btc}
}

func (b *btcClientWithMetrics) GetTipHeight() (uint64, error) {
	return runBtcClientMethodWithMetrics("GetTipHeight", b.btc.GetTipHeight)
}

func (b *btcClientWithMetrics) GetBlockTimestamp(height uint32) (int64, error) {
	return runBtcClientMethodWithMetrics("GetBlockTimestamp", func() (int64, error) {
		return b.btc.GetBlockTimestamp(height)
	})
}

func runBtcClientMethodWithMetrics[T any](method string, f func() (T, error)) (T, error) {
	startTime := time.Now()
	v, err := f()
	duration := time.Since(startTime)

	metrics.RecordBTCClientLatency(duration, method, err != nil)
	return v, err
}
