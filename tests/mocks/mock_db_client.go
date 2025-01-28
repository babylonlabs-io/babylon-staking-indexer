// Code generated by mockery v2.44.1. DO NOT EDIT.

package mocks

import (
	context "context"

	bbnclient "github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"

	db "github.com/babylonlabs-io/babylon-staking-indexer/internal/db"

	mock "github.com/stretchr/testify/mock"

	model "github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"

	types "github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
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

// FindExpiredDelegations provides a mock function with given fields: ctx, btcTipHeight, limit
func (_m *DbInterface) FindExpiredDelegations(ctx context.Context, btcTipHeight uint64, limit uint64) ([]model.TimeLockDocument, error) {
	ret := _m.Called(ctx, btcTipHeight, limit)

	if len(ret) == 0 {
		panic("no return value specified for FindExpiredDelegations")
	}

	var r0 []model.TimeLockDocument
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64, uint64) ([]model.TimeLockDocument, error)); ok {
		return rf(ctx, btcTipHeight, limit)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64, uint64) []model.TimeLockDocument); ok {
		r0 = rf(ctx, btcTipHeight, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]model.TimeLockDocument)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64, uint64) error); ok {
		r1 = rf(ctx, btcTipHeight, limit)
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

// GetBTCDelegationState provides a mock function with given fields: ctx, stakingTxHash
func (_m *DbInterface) GetBTCDelegationState(ctx context.Context, stakingTxHash string) (*types.DelegationState, error) {
	ret := _m.Called(ctx, stakingTxHash)

	if len(ret) == 0 {
		panic("no return value specified for GetBTCDelegationState")
	}

	var r0 *types.DelegationState
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*types.DelegationState, error)); ok {
		return rf(ctx, stakingTxHash)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.DelegationState); ok {
		r0 = rf(ctx, stakingTxHash)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.DelegationState)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, stakingTxHash)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBTCDelegationsByStates provides a mock function with given fields: ctx, states
func (_m *DbInterface) GetBTCDelegationsByStates(ctx context.Context, states []types.DelegationState) ([]*model.BTCDelegationDetails, error) {
	ret := _m.Called(ctx, states)

	if len(ret) == 0 {
		panic("no return value specified for GetBTCDelegationsByStates")
	}

	var r0 []*model.BTCDelegationDetails
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []types.DelegationState) ([]*model.BTCDelegationDetails, error)); ok {
		return rf(ctx, states)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []types.DelegationState) []*model.BTCDelegationDetails); ok {
		r0 = rf(ctx, states)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.BTCDelegationDetails)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []types.DelegationState) error); ok {
		r1 = rf(ctx, states)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDelegationsByFinalityProvider provides a mock function with given fields: ctx, fpBtcPkHex
func (_m *DbInterface) GetDelegationsByFinalityProvider(ctx context.Context, fpBtcPkHex string) ([]*model.BTCDelegationDetails, error) {
	ret := _m.Called(ctx, fpBtcPkHex)

	if len(ret) == 0 {
		panic("no return value specified for GetDelegationsByFinalityProvider")
	}

	var r0 []*model.BTCDelegationDetails
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]*model.BTCDelegationDetails, error)); ok {
		return rf(ctx, fpBtcPkHex)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []*model.BTCDelegationDetails); ok {
		r0 = rf(ctx, fpBtcPkHex)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.BTCDelegationDetails)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, fpBtcPkHex)
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

// GetLastProcessedBbnHeight provides a mock function with given fields: ctx
func (_m *DbInterface) GetLastProcessedBbnHeight(ctx context.Context) (uint64, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetLastProcessedBbnHeight")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (uint64, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) uint64); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStakingParams provides a mock function with given fields: ctx, version
func (_m *DbInterface) GetStakingParams(ctx context.Context, version uint32) (*bbnclient.StakingParams, error) {
	ret := _m.Called(ctx, version)

	if len(ret) == 0 {
		panic("no return value specified for GetStakingParams")
	}

	var r0 *bbnclient.StakingParams
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uint32) (*bbnclient.StakingParams, error)); ok {
		return rf(ctx, version)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint32) *bbnclient.StakingParams); ok {
		r0 = rf(ctx, version)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*bbnclient.StakingParams)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint32) error); ok {
		r1 = rf(ctx, version)
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

// SaveBTCDelegationSlashingTxHex provides a mock function with given fields: ctx, stakingTxHashHex, slashingTxHex, spendingHeight
func (_m *DbInterface) SaveBTCDelegationSlashingTxHex(ctx context.Context, stakingTxHashHex string, slashingTxHex string, spendingHeight uint32) error {
	ret := _m.Called(ctx, stakingTxHashHex, slashingTxHex, spendingHeight)

	if len(ret) == 0 {
		panic("no return value specified for SaveBTCDelegationSlashingTxHex")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, uint32) error); ok {
		r0 = rf(ctx, stakingTxHashHex, slashingTxHex, spendingHeight)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveBTCDelegationUnbondingCovenantSignature provides a mock function with given fields: ctx, stakingTxHash, covenantBtcPkHex, signatureHex
func (_m *DbInterface) SaveBTCDelegationUnbondingCovenantSignature(ctx context.Context, stakingTxHash string, covenantBtcPkHex string, signatureHex string) error {
	ret := _m.Called(ctx, stakingTxHash, covenantBtcPkHex, signatureHex)

	if len(ret) == 0 {
		panic("no return value specified for SaveBTCDelegationUnbondingCovenantSignature")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) error); ok {
		r0 = rf(ctx, stakingTxHash, covenantBtcPkHex, signatureHex)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveBTCDelegationUnbondingSlashingTxHex provides a mock function with given fields: ctx, stakingTxHashHex, unbondingSlashingTxHex, spendingHeight
func (_m *DbInterface) SaveBTCDelegationUnbondingSlashingTxHex(ctx context.Context, stakingTxHashHex string, unbondingSlashingTxHex string, spendingHeight uint32) error {
	ret := _m.Called(ctx, stakingTxHashHex, unbondingSlashingTxHex, spendingHeight)

	if len(ret) == 0 {
		panic("no return value specified for SaveBTCDelegationUnbondingSlashingTxHex")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, uint32) error); ok {
		r0 = rf(ctx, stakingTxHashHex, unbondingSlashingTxHex, spendingHeight)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveCheckpointParams provides a mock function with given fields: ctx, params
func (_m *DbInterface) SaveCheckpointParams(ctx context.Context, params *bbnclient.CheckpointParams) error {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for SaveCheckpointParams")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *bbnclient.CheckpointParams) error); ok {
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

// SaveNewTimeLockExpire provides a mock function with given fields: ctx, stakingTxHashHex, expireHeight, subState
func (_m *DbInterface) SaveNewTimeLockExpire(ctx context.Context, stakingTxHashHex string, expireHeight uint32, subState types.DelegationSubState) error {
	ret := _m.Called(ctx, stakingTxHashHex, expireHeight, subState)

	if len(ret) == 0 {
		panic("no return value specified for SaveNewTimeLockExpire")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, uint32, types.DelegationSubState) error); ok {
		r0 = rf(ctx, stakingTxHashHex, expireHeight, subState)
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

// UpdateBTCDelegationState provides a mock function with given fields: ctx, stakingTxHash, qualifiedPreviousStates, newState, opts
func (_m *DbInterface) UpdateBTCDelegationState(ctx context.Context, stakingTxHash string, qualifiedPreviousStates []types.DelegationState, newState types.DelegationState, opts ...db.UpdateOption) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, stakingTxHash, qualifiedPreviousStates, newState)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for UpdateBTCDelegationState")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []types.DelegationState, types.DelegationState, ...db.UpdateOption) error); ok {
		r0 = rf(ctx, stakingTxHash, qualifiedPreviousStates, newState, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateDelegationsStateByFinalityProvider provides a mock function with given fields: ctx, fpBtcPkHex, newState, bbnBlockHeight
func (_m *DbInterface) UpdateDelegationsStateByFinalityProvider(ctx context.Context, fpBtcPkHex string, newState types.DelegationState, bbnBlockHeight int64) error {
	ret := _m.Called(ctx, fpBtcPkHex, newState, bbnBlockHeight)

	if len(ret) == 0 {
		panic("no return value specified for UpdateDelegationsStateByFinalityProvider")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, types.DelegationState, int64) error); ok {
		r0 = rf(ctx, fpBtcPkHex, newState, bbnBlockHeight)
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

// UpdateLastProcessedBbnHeight provides a mock function with given fields: ctx, height
func (_m *DbInterface) UpdateLastProcessedBbnHeight(ctx context.Context, height uint64) error {
	ret := _m.Called(ctx, height)

	if len(ret) == 0 {
		panic("no return value specified for UpdateLastProcessedBbnHeight")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64) error); ok {
		r0 = rf(ctx, height)
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
