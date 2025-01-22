//go:build integration

package db_test

import (
	"context"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestTimeLock(t *testing.T) {
	ctx := context.Background() // todo (Kirill) change to t.Ctx() after go1.24
	t.Cleanup(func() {
		resetDatabase(t)
	})
	t.Run("no documents", func(t *testing.T) {
		docs, err := testDB.FindExpiredDelegations(ctx, math.MaxInt64, 10)
		require.NoError(t, err)
		assert.Nil(t, docs)
	})
	t.Run("find documents", func(t *testing.T) {
		expiredDelegation1 := model.TimeLockDocument{
			StakingTxHashHex:   utils.RandomAlphaNum(10),
			ExpireHeight:       1,
			DelegationSubState: types.SubStateTimelock,
		}
		expiredDelegation2 := model.TimeLockDocument{
			StakingTxHashHex:   utils.RandomAlphaNum(10),
			ExpireHeight:       5,
			DelegationSubState: types.SubStateTimelock,
		}
		nonExpiredDelegation := model.TimeLockDocument{
			StakingTxHashHex:   utils.RandomAlphaNum(10),
			ExpireHeight:       10,
			DelegationSubState: types.SubStateTimelock,
		}

		docs := []model.TimeLockDocument{expiredDelegation1, expiredDelegation2, nonExpiredDelegation}
		for _, doc := range docs {
			err := testDB.SaveNewTimeLockExpire(ctx, doc.StakingTxHashHex, doc.ExpireHeight, doc.DelegationSubState)
			require.NoError(t, err)
		}

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
}
