package services

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/avast/retry-go/v4"
	"time"
)

const (
	retryInitialDelay    = time.Second // initial delay between attempts
	retryMaxAllowedDelay = 10 * time.Second

	btcNotifierMaxRetries = 3
)

//go:generate mockery --name=BtcNotifier --output=../../tests/mocks --outpkg=mocks --filename=mock_btc_notifier.go
type BtcNotifier interface {
	Start() error
	RegisterSpendNtfn(outpoint *wire.OutPoint, pkScript []byte, heightHint uint32) (*chainntnfs.SpendEvent, error)
}

// btcNotifierWithRetries is a wrapper around a BtcNotifier
// that retries all methods except Start() for maxRetries times
type btcNotifierWithRetries struct {
	notifier   BtcNotifier
	maxRetries int
}

func newBtcNotifierWithRetries(notifier BtcNotifier) *btcNotifierWithRetries {
	return &btcNotifierWithRetries{
		notifier:   notifier,
		maxRetries: btcNotifierMaxRetries,
	}
}

func (b *btcNotifierWithRetries) Start() error {
	return b.notifier.Start()
}

func (b *btcNotifierWithRetries) RegisterSpendNtfn(outpoint *wire.OutPoint, pkScript []byte, heightHint uint32) (*chainntnfs.SpendEvent, error) {
	f := func() (*chainntnfs.SpendEvent, error) {
		return b.notifier.RegisterSpendNtfn(outpoint, pkScript, heightHint)
	}

	// by default exponential delay is going to be used
	result, err := retry.DoWithData(
		f,
		retry.Attempts(uint(b.maxRetries)),
		retry.Delay(retryInitialDelay),
		retry.MaxDelay(retryMaxAllowedDelay),
	)

	return result, err
}
