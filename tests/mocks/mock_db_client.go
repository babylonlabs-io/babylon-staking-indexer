// Code generated by mockery v2.42.1. DO NOT EDIT.

package mocks

import (
	context "context"

	bbnclient "github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"

	internaltypes "github.com/babylonlabs-io/babylon-staking-indexer/internal/types"

	mock "github.com/stretchr/testify/mock"

	model "github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"

	types "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
)

// DbInterface is an autogenerated mock type for the DbInterface type
type DbInterface struct {
	mock.Mock
}

// DeleteExpiredDelegation provides a mock function with given fields: ctx, stakingTxHashHex
func (_m *DbInterface) DeleteExpiredDelegation(ctx context.Context, stakingTxHashHex string) error {
	ret := _m.Called(ctx, stakingTxHashHex)

	if len(ret) == 0 {
		panic("no return value specified for DeleteExpiredDelegation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, stakingTxHashHex)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FindExpiredDelegations provides a mock function with given fields: ctx, btcTipHeight
func (_m *DbInterface) FindExpiredDelegations(ctx context.Context, btcTipHeight uint64) ([]model.TimeLockDocument, error) {
	ret := _m.Called(ctx, btcTipHeight)

	if len(ret) == 0 {
		panic("no return value specified for FindExpiredDelegations")
	}

	var r0 []model.TimeLockDocument
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64) ([]model.TimeLockDocument, error)); ok {
		return rf(ctx, btcTipHeight)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64) []model.TimeLockDocument); ok {
		r0 = rf(ctx, btcTipHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]model.TimeLockDocument)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64) error); ok {
		r1 = rf(ctx, btcTipHeight)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBTCDelegationByStakingTxHash provides a mock function with given fields: ctx, stakingTxHash
func (_m *DbInterface) GetBTCDelegationByStakingTxHash(ctx context.Context, stakingTxHash string) (*model.BTCDelegationDetails, error) {
	ret := _m.Called(ctx, stakingTxHash)

	if len(ret) == 0 {
		panic("no return value specified for GetBTCDelegationByStakingTxHash")
	}

	var r0 *model.BTCDelegationDetails
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*model.BTCDelegationDetails, error)); ok {
		return rf(ctx, stakingTxHash)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.BTCDelegationDetails); ok {
		r0 = rf(ctx, stakingTxHash)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.BTCDelegationDetails)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, stakingTxHash)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetFinalityProviderByBtcPk provides a mock function with given fields: ctx, btcPk
func (_m *DbInterface) GetFinalityProviderByBtcPk(ctx context.Context, btcPk string) (*model.FinalityProviderDetails, error) {
	ret := _m.Called(ctx, btcPk)

	if len(ret) == 0 {
		panic("no return value specified for GetFinalityProviderByBtcPk")
	}

	var r0 *model.FinalityProviderDetails
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*model.FinalityProviderDetails, error)); ok {
		return rf(ctx, btcPk)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.FinalityProviderDetails); ok {
		r0 = rf(ctx, btcPk)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.FinalityProviderDetails)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, btcPk)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ping provides a mock function with given fields: ctx
func (_m *DbInterface) Ping(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Ping")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveCheckpointParams provides a mock function with given fields: ctx, params
func (_m *DbInterface) SaveCheckpointParams(ctx context.Context, params *types.Params) error {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for SaveCheckpointParams")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Params) error); ok {
		r0 = rf(ctx, params)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveNewBTCDelegation provides a mock function with given fields: ctx, delegationDoc
func (_m *DbInterface) SaveNewBTCDelegation(ctx context.Context, delegationDoc *model.BTCDelegationDetails) error {
	ret := _m.Called(ctx, delegationDoc)

	if len(ret) == 0 {
		panic("no return value specified for SaveNewBTCDelegation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *model.BTCDelegationDetails) error); ok {
		r0 = rf(ctx, delegationDoc)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveNewFinalityProvider provides a mock function with given fields: ctx, fpDoc
func (_m *DbInterface) SaveNewFinalityProvider(ctx context.Context, fpDoc *model.FinalityProviderDetails) error {
	ret := _m.Called(ctx, fpDoc)

	if len(ret) == 0 {
		panic("no return value specified for SaveNewFinalityProvider")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *model.FinalityProviderDetails) error); ok {
		r0 = rf(ctx, fpDoc)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveNewTimeLockExpire provides a mock function with given fields: ctx, stakingTxHashHex, expireHeight, txType
func (_m *DbInterface) SaveNewTimeLockExpire(ctx context.Context, stakingTxHashHex string, expireHeight uint32, txType string) error {
	ret := _m.Called(ctx, stakingTxHashHex, expireHeight, txType)

	if len(ret) == 0 {
		panic("no return value specified for SaveNewTimeLockExpire")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, uint32, string) error); ok {
		r0 = rf(ctx, stakingTxHashHex, expireHeight, txType)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveStakingParams provides a mock function with given fields: ctx, version, params
func (_m *DbInterface) SaveStakingParams(ctx context.Context, version uint32, params *bbnclient.StakingParams) error {
	ret := _m.Called(ctx, version, params)

	if len(ret) == 0 {
		panic("no return value specified for SaveStakingParams")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uint32, *bbnclient.StakingParams) error); ok {
		r0 = rf(ctx, version, params)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateBTCDelegationDetails provides a mock function with given fields: ctx, stakingTxHash, details
func (_m *DbInterface) UpdateBTCDelegationDetails(ctx context.Context, stakingTxHash string, details *model.BTCDelegationDetails) error {
	ret := _m.Called(ctx, stakingTxHash, details)

	if len(ret) == 0 {
		panic("no return value specified for UpdateBTCDelegationDetails")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *model.BTCDelegationDetails) error); ok {
		r0 = rf(ctx, stakingTxHash, details)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateBTCDelegationState provides a mock function with given fields: ctx, stakingTxHash, newState
func (_m *DbInterface) UpdateBTCDelegationState(ctx context.Context, stakingTxHash string, newState internaltypes.DelegationState) error {
	ret := _m.Called(ctx, stakingTxHash, newState)

	if len(ret) == 0 {
		panic("no return value specified for UpdateBTCDelegationState")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, internaltypes.DelegationState) error); ok {
		r0 = rf(ctx, stakingTxHash, newState)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateFinalityProviderDetailsFromEvent provides a mock function with given fields: ctx, detailsToUpdate
func (_m *DbInterface) UpdateFinalityProviderDetailsFromEvent(ctx context.Context, detailsToUpdate *model.FinalityProviderDetails) error {
	ret := _m.Called(ctx, detailsToUpdate)

	if len(ret) == 0 {
		panic("no return value specified for UpdateFinalityProviderDetailsFromEvent")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *model.FinalityProviderDetails) error); ok {
		r0 = rf(ctx, detailsToUpdate)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateFinalityProviderState provides a mock function with given fields: ctx, btcPk, newState
func (_m *DbInterface) UpdateFinalityProviderState(ctx context.Context, btcPk string, newState string) error {
	ret := _m.Called(ctx, btcPk, newState)

	if len(ret) == 0 {
		panic("no return value specified for UpdateFinalityProviderState")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, btcPk, newState)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewDbInterface creates a new instance of DbInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDbInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *DbInterface {
	mock := &DbInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
