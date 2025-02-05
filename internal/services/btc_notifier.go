package services

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"errors"
)

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

func newBtcNotifierWithRetries(notifier BtcNotifier, maxRetries int) *btcNotifierWithRetries {
	return &btcNotifierWithRetries{
		notifier:   notifier,
		maxRetries: maxRetries,
	}
}

func (b *btcNotifierWithRetries) Start() error {
	return b.notifier.Start()
}

func (b *btcNotifierWithRetries) RegisterSpendNtfn(outpoint *wire.OutPoint, pkScript []byte, heightHint uint32) (*chainntnfs.SpendEvent, error) {
	var errs []error
	for i := 0; i < b.maxRetries; i++ {
		result, err := b.notifier.RegisterSpendNtfn(outpoint, pkScript, heightHint)
		if err == nil {
			return result, nil
		}

		errs = append(errs, err)
	}

	return nil, errors.Join(errs...)
}
