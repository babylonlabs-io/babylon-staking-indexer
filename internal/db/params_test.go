//go:build integration

package db_test

import (
	"math"
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestParams(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})
	t.Run("staking params", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			const version = math.MaxUint32

			params := &bbnclient.StakingParams{
				CovenantQuorum:     111,
				MinStakingValueSat: 10,
			}
			err := testDB.SaveStakingParams(ctx, version, params)
			require.NoError(t, err)

			actualParams, err := testDB.GetStakingParams(ctx, version)
			require.NoError(t, err)
			assert.Equal(t, params, actualParams)
		})
		t.Run("insert duplicate", func(t *testing.T) {
			const version = 1
			params := &bbnclient.StakingParams{
				CovenantQuorum:     123,
				MinStakingValueSat: 23,
			}

			err := testDB.SaveStakingParams(ctx, version, params)
			require.NoError(t, err)

			err = testDB.SaveStakingParams(ctx, version, params)
			assert.True(t, db.IsDuplicateKeyError(err))
		})
	})
	t.Run("checkpoint params", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			params, err := testDB.GetCheckpointParams(ctx)
			assert.ErrorIs(t, err, mongo.ErrNoDocuments)
			assert.Nil(t, params)
		})
		t.Run("check upsert", func(t *testing.T) {
			updates := []*bbnclient.CheckpointParams{
				{
					BtcConfirmationDepth:          10,
					CheckpointFinalizationTimeout: 100,
					CheckpointTag:                 "62627435",
				},
				{
					BtcConfirmationDepth:          30,
					CheckpointFinalizationTimeout: 100,
					CheckpointTag:                 "62627435",
				},
			}

			// on first iteration we check insertion
			// on second we check that update has been applied
			for _, update := range updates {
				err := testDB.SaveCheckpointParams(ctx, update)
				require.NoError(t, err)

				params, err := testDB.GetCheckpointParams(ctx)
				require.NoError(t, err)
				assert.Equal(t, update, params)
			}
		})
	})
	t.Run("update max finality providers", func(t *testing.T) {
		const version = 0

		initialParams := &bbnclient.StakingParams{
			CovenantQuorum:       111,
			MinStakingValueSat:   10,
			MaxFinalityProviders: 0,
		}
		err := testDB.SaveStakingParams(ctx, version, initialParams)
		require.NoError(t, err)

		const maxFinalityProviders = 77
		err = testDB.UpdateStakingParamMaxFinalityProviders(ctx, version, maxFinalityProviders)
		require.NoError(t, err)

		params, err := testDB.GetStakingParams(ctx, version)
		require.NoError(t, err)

		initialParams.MaxFinalityProviders = maxFinalityProviders
		assert.Equal(t, initialParams, params)
	})
}
