package services

import (
	"testing"
	"github.com/babylonlabs-io/babylon-staking-indexer/tests/mocks"
	"github.com/stretchr/testify/require"
	"errors"
	"github.com/stretchr/testify/mock"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/stretchr/testify/assert"
)

func TestBTCNotifier_Start(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		notifier := mocks.NewBtcNotifier(t)
		wrapper := newBtcNotifierWithRetries(notifier)

		notifier.On("Start").Return(nil).Once()
		err := wrapper.Start()
		require.NoError(t, err)
	})
	t.Run("failure", func(t *testing.T) {
		notifier := mocks.NewBtcNotifier(t)
		wrapper := newBtcNotifierWithRetries(notifier)

		someErr := errors.New("some err")

		notifier.On("Start").Return(someErr).Once()
		err := wrapper.Start()
		require.ErrorIs(t, err, someErr)
	})
}

func TestBTCNotifier_RegisterSpendNtfn(t *testing.T) {
	// todo (https://github.com/babylonlabs-io/babylon-staking-indexer/issues/161) after upgrade to go 1.24 use synctest package to speed up test
	// the most important thing in these tests is number of expected calls to mock (.Once() or .Times())
	// that essentially allows us to test logic of retries
	t.Run("ok", func(t *testing.T) {
		notifier := mocks.NewBtcNotifier(t)
		wrapper := newBtcNotifierWithRetries(notifier)

		event := new(chainntnfs.SpendEvent)
		notifier.On("RegisterSpendNtfn", mock.Anything, mock.Anything, mock.Anything).Return(event, nil).Once()
		// actual values are irrelevant
		v, err := wrapper.RegisterSpendNtfn(nil, nil, 33)
		require.NoError(t, err)
		assert.Equal(t, event, v)
	})
	t.Run("1 failure", func(t *testing.T) {
		notifier := mocks.NewBtcNotifier(t)
		wrapper := newBtcNotifierWithRetries(notifier)

		someErr := errors.New("some error")

		notifier.On("RegisterSpendNtfn", mock.Anything, mock.Anything, mock.Anything).Return(nil, someErr).Once()
		event := new(chainntnfs.SpendEvent)
		notifier.On("RegisterSpendNtfn", mock.Anything, mock.Anything, mock.Anything).Return(event, nil).Once()
		// actual values are irrelevant
		v, err := wrapper.RegisterSpendNtfn(nil, nil, 33)
		require.NoError(t, err)
		assert.Equal(t, event, v)
	})
	t.Run("all failures", func(t *testing.T) {
		notifier := mocks.NewBtcNotifier(t)
		wrapper := newBtcNotifierWithRetries(notifier)

		someErr := errors.New("some error")

		notifier.On("RegisterSpendNtfn", mock.Anything, mock.Anything, mock.Anything).Return(nil, someErr).Times(btcNotifierMaxRetries)
		// actual values are irrelevant
		_, err := wrapper.RegisterSpendNtfn(nil, nil, 33)
		require.Error(t, err)
	})
}
