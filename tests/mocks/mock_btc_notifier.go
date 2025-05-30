// Code generated by mockery v2.51.0. DO NOT EDIT.

package mocks

import (
	chainntnfs "github.com/lightningnetwork/lnd/chainntnfs"
	mock "github.com/stretchr/testify/mock"

	wire "github.com/btcsuite/btcd/wire"
)

// BtcNotifier is an autogenerated mock type for the BtcNotifier type
type BtcNotifier struct {
	mock.Mock
}

// RegisterSpendNtfn provides a mock function with given fields: outpoint, pkScript, heightHint
func (_m *BtcNotifier) RegisterSpendNtfn(outpoint *wire.OutPoint, pkScript []byte, heightHint uint32) (*chainntnfs.SpendEvent, error) {
	ret := _m.Called(outpoint, pkScript, heightHint)

	if len(ret) == 0 {
		panic("no return value specified for RegisterSpendNtfn")
	}

	var r0 *chainntnfs.SpendEvent
	var r1 error
	if rf, ok := ret.Get(0).(func(*wire.OutPoint, []byte, uint32) (*chainntnfs.SpendEvent, error)); ok {
		return rf(outpoint, pkScript, heightHint)
	}
	if rf, ok := ret.Get(0).(func(*wire.OutPoint, []byte, uint32) *chainntnfs.SpendEvent); ok {
		r0 = rf(outpoint, pkScript, heightHint)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*chainntnfs.SpendEvent)
		}
	}

	if rf, ok := ret.Get(1).(func(*wire.OutPoint, []byte, uint32) error); ok {
		r1 = rf(outpoint, pkScript, heightHint)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Start provides a mock function with no fields
func (_m *BtcNotifier) Start() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewBtcNotifier creates a new instance of BtcNotifier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewBtcNotifier(t interface {
	mock.TestingT
	Cleanup(func())
}) *BtcNotifier {
	mock := &BtcNotifier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
