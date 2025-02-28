package db

import (
	"context"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"time"
)

type DbWithMetrics struct {
	db DbInterface
}

func NewDbWithMetrics(db DbInterface) *DbWithMetrics {
	return &DbWithMetrics{db: db}
}

func (d *DbWithMetrics) Ping(ctx context.Context) error {
	return d.db.Ping(ctx)
}

func (d *DbWithMetrics) SaveNewFinalityProvider(ctx context.Context, fpDoc *model.FinalityProviderDetails) error {
	return d.run("SaveNewFinalityProvider", func() error {
		return d.db.SaveNewFinalityProvider(ctx, fpDoc)
	})
}

func (d *DbWithMetrics) UpdateFinalityProviderState(ctx context.Context, btcPk string, newState string) error {
	return d.run("UpdateFinalityProviderState", func() error {
		return d.db.UpdateFinalityProviderState(ctx, btcPk, newState)
	})
}

func (d *DbWithMetrics) UpdateFinalityProviderDetailsFromEvent(ctx context.Context, detailsToUpdate *model.FinalityProviderDetails) error {
	return d.run("UpdateFinalityProviderDetailsFromEvent", func() error {
		return d.db.UpdateFinalityProviderDetailsFromEvent(ctx, detailsToUpdate)
	})
}

func (d *DbWithMetrics) GetFinalityProviderByBtcPk(ctx context.Context, btcPk string) (result *model.FinalityProviderDetails, err error) {
	//nolint:errcheck
	d.run("GetFinalityProviderByBtcPk", func() error {
		result, err = d.db.GetFinalityProviderByBtcPk(ctx, btcPk)
		return err
	})

	return
}

func (d *DbWithMetrics) SaveStakingParams(ctx context.Context, version uint32, params *bbnclient.StakingParams) error {
	return d.run("SaveStakingParams", func() error {
		return d.db.SaveStakingParams(ctx, version, params)
	})
}

func (d *DbWithMetrics) GetStakingParams(ctx context.Context, version uint32) (result *bbnclient.StakingParams, err error) {
	//nolint:errcheck
	d.run("GetStakingParams", func() error {
		result, err = d.db.GetStakingParams(ctx, version)
		return err
	})
	return
}

func (d *DbWithMetrics) SaveCheckpointParams(ctx context.Context, params *bbnclient.CheckpointParams) error {
	return d.run("SaveCheckpointParams", func() error {
		return d.db.SaveCheckpointParams(ctx, params)
	})
}

func (d *DbWithMetrics) SaveNewBTCDelegation(ctx context.Context, delegationDoc *model.BTCDelegationDetails) error {
	return d.run("SaveNewBTCDelegation", func() error {
		return d.db.SaveNewBTCDelegation(ctx, delegationDoc)
	})
}

func (d *DbWithMetrics) UpdateBTCDelegationState(ctx context.Context, stakingTxHash string, qualifiedPreviousStates []types.DelegationState, newState types.DelegationState, opts ...UpdateOption) error {
	return d.run("UpdateBTCDelegationState", func() error {
		return d.db.UpdateBTCDelegationState(ctx, stakingTxHash, qualifiedPreviousStates, newState, opts...)
	})
}

func (d *DbWithMetrics) SaveBTCDelegationUnbondingCovenantSignature(ctx context.Context, stakingTxHash string, covenantBtcPkHex string, signatureHex string) error {
	return d.run("SaveBTCDelegationUnbondingCovenantSignature", func() error {
		return d.db.SaveBTCDelegationUnbondingCovenantSignature(ctx, stakingTxHash, covenantBtcPkHex, signatureHex)
	})
}

func (d *DbWithMetrics) GetBTCDelegationState(ctx context.Context, stakingTxHash string) (result *types.DelegationState, err error) {
	//nolint:errcheck
	d.run("GetBTCDelegationState", func() error {
		result, err = d.db.GetBTCDelegationState(ctx, stakingTxHash)
		return err
	})
	return
}

func (d *DbWithMetrics) GetBTCDelegationByStakingTxHash(ctx context.Context, stakingTxHash string) (result *model.BTCDelegationDetails, err error) {
	//nolint:errcheck
	d.run("GetBTCDelegationByStakingTxHash", func() error {
		result, err = d.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
		return err
	})
	return
}

func (d *DbWithMetrics) GetDelegationsByFinalityProvider(ctx context.Context, fpBtcPkHex string) (result []*model.BTCDelegationDetails, err error) {
	//nolint:errcheck
	d.run("GetDelegationsByFinalityProvider", func() error {
		result, err = d.db.GetDelegationsByFinalityProvider(ctx, fpBtcPkHex)
		return err
	})
	return
}

func (d *DbWithMetrics) SaveNewTimeLockExpire(ctx context.Context, stakingTxHashHex string, expireHeight uint32, subState types.DelegationSubState) error {
	return d.run("SaveNewTimeLockExpire", func() error {
		return d.db.SaveNewTimeLockExpire(ctx, stakingTxHashHex, expireHeight, subState)
	})
}

func (d *DbWithMetrics) FindExpiredDelegations(ctx context.Context, btcTipHeight, limit uint64) (result []model.TimeLockDocument, err error) {
	//nolint:errcheck
	d.run("FindExpiredDelegations", func() error {
		result, err = d.db.FindExpiredDelegations(ctx, btcTipHeight, limit)
		return err
	})
	return
}

func (d *DbWithMetrics) DeleteExpiredDelegation(ctx context.Context, stakingTxHashHex string) error {
	return d.run("DeleteExpiredDelegation", func() error {
		return d.db.DeleteExpiredDelegation(ctx, stakingTxHashHex)
	})
}

func (d *DbWithMetrics) GetLastProcessedBbnHeight(ctx context.Context) (result uint64, err error) {
	//nolint:errcheck
	d.run("GetLastProcessedBbnHeight", func() error {
		result, err = d.db.GetLastProcessedBbnHeight(ctx)
		return err
	})
	return
}

func (d *DbWithMetrics) UpdateLastProcessedBbnHeight(ctx context.Context, height uint64) error {
	return d.run("UpdateLastProcessedBbnHeight", func() error {
		return d.db.UpdateLastProcessedBbnHeight(ctx, height)
	})
}

func (d *DbWithMetrics) GetBTCDelegationsByStates(ctx context.Context, states []types.DelegationState) (result []*model.BTCDelegationDetails, err error) {
	//nolint:errcheck
	d.run("GetBTCDelegationsByStates", func() error {
		result, err = d.db.GetBTCDelegationsByStates(ctx, states)
		return err
	})
	return
}

func (d *DbWithMetrics) UpdateDelegationStakerBabylonAddress(ctx context.Context, stakingTxHash, stakerBabylonAddress string) error {
	return d.run("UpdateDelegationStakerBabylonAddress", func() error {
		return d.db.UpdateDelegationStakerBabylonAddress(ctx, stakingTxHash, stakerBabylonAddress)
	})
}

// run is private method that executes passed lambda function and send metrics data with spent time, method name
// and an error if any. It returns the error from the lambda function for convenience
func (d *DbWithMetrics) run(method string, f func() error) error {
	startTime := time.Now()
	err := f()
	duration := time.Since(startTime)

	metrics.RecordDbLatency(duration, method, err != nil)
	return err
}
