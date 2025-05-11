//go:build integration

package db_test

import (
	"math"
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeLock(t *testing.T) {
	ctx := t.Context()
	t.Run("no documents", func(t *testing.T) {
		docs, err := testDB.FindExpiredDelegations(ctx, math.MaxInt64, 10)
		require.NoError(t, err)
		assert.Nil(t, docs)
	})
	t.Run("find documents", func(t *testing.T) {
		expiredDelegation1 := model.TimeLockDocument{
			StakingTxHashHex:   randomStakingTxHashHex(t),
			ExpireHeight:       1,
			DelegationSubState: types.SubStateTimelock,
		}
		expiredDelegation2 := model.TimeLockDocument{
			StakingTxHashHex:   randomStakingTxHashHex(t),
			ExpireHeight:       5,
			DelegationSubState: types.SubStateTimelock,
		}

		nonExpiredDelegation := model.TimeLockDocument{
			StakingTxHashHex:   randomStakingTxHashHex(t),
			ExpireHeight:       10,
			DelegationSubState: types.SubStateTimelock,
		}

		docs := []model.TimeLockDocument{expiredDelegation1, expiredDelegation2, nonExpiredDelegation}
		for _, doc := range docs {
			err := testDB.SaveNewTimeLockExpire(ctx, doc.StakingTxHashHex, doc.ExpireHeight, doc.DelegationSubState)
			require.NoError(t, err)
		}
		defer resetDatabase(t)

		// by choosing exactly the same expire height we test equal part of lte query
		btcTipHeight := expiredDelegation2.ExpireHeight
		// just to prevent accidental test failures on test rewrite
		// double check that expiredDelegation1 ExpireHeight field is less than chosen btcTipHeight
		require.Less(t, expiredDelegation1.ExpireHeight, btcTipHeight)

		docs, err := testDB.FindExpiredDelegations(ctx, uint64(btcTipHeight), 10)
		require.NoError(t, err)

		expectedDocs := []model.TimeLockDocument{expiredDelegation1, expiredDelegation2}
		assert.Equal(t, expectedDocs, docs)
	})
	t.Run("delete", func(t *testing.T) {
		// first check deletion of non existing delegation
		err := testDB.DeleteExpiredDelegation(ctx, randomStakingTxHashHex(t))
		assert.Error(t, err)

		// main path
		doc := model.TimeLockDocument{
			StakingTxHashHex:   randomStakingTxHashHex(t),
			ExpireHeight:       10,
			DelegationSubState: types.SubStateTimelock,
		}
		err = testDB.SaveNewTimeLockExpire(ctx, doc.StakingTxHashHex, doc.ExpireHeight, doc.DelegationSubState)
		require.NoError(t, err)

		err = testDB.DeleteExpiredDelegation(ctx, doc.StakingTxHashHex)
		require.NoError(t, err)

		docs, err := testDB.FindExpiredDelegations(ctx, uint64(doc.ExpireHeight+1), 1)
		require.NoError(t, err)
		require.Empty(t, docs)
	})
}

func randomStakingTxHashHex(t *testing.T) string {
	result, err := testutil.RandomAlphaNum(10)
	require.NoError(t, err)

	return result
}
