//go:build integration

package db_test

import (
	"context"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestParams(t *testing.T) {
	ctx := context.Background() // todo (Kirill) change to t.Ctx() after go1.24
	t.Cleanup(func() {
		resetDatabase(t)
	})
	t.Run("staking params", func(t *testing.T) {
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
	t.Run("checkpoint params", func(t *testing.T) {
		err := testDB.SaveCheckpointParams(ctx, &bbnclient.CheckpointParams{})
		require.NoError(t, err)
	})
}
