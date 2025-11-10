//go:build integration

package db_test

import (
	"strings"
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	t.Run("CalculateActiveStatsAggregated - no delegations", func(t *testing.T) {
		// When no delegations exist, should return zeros and empty array
		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), tvl)
		assert.Equal(t, uint64(0), delegations)
		assert.Empty(t, fpStats)
	})

	t.Run("CalculateActiveStatsAggregated - only non-active delegations", func(t *testing.T) {
		// Create delegations with non-active states
		delegation1 := createDelegation(t)
		delegation1.State = types.StatePending
		delegation1.StakingAmount = 10000
		err := testDB.SaveNewBTCDelegation(ctx, delegation1)
		require.NoError(t, err)

		delegation2 := createDelegation(t)
		delegation2.State = types.StateWithdrawn
		delegation2.StakingAmount = 20000
		err = testDB.SaveNewBTCDelegation(ctx, delegation2)
		require.NoError(t, err)

		// Should return zeros since only ACTIVE delegations are counted
		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), tvl)
		assert.Equal(t, uint64(0), delegations)
		assert.Empty(t, fpStats)
	})

	t.Run("CalculateActiveStatsAggregated - single active delegation", func(t *testing.T) {
		resetDatabase(t)

		fpPk := randomBTCpk(t)
		delegation := createDelegation(t)
		delegation.State = types.StateActive
		delegation.StakingAmount = 100000
		delegation.FinalityProviderBtcPksHex = []string{fpPk}
		err := testDB.SaveNewBTCDelegation(ctx, delegation)
		require.NoError(t, err)

		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(100000), tvl)
		assert.Equal(t, uint64(1), delegations)
		require.Len(t, fpStats, 1)
		// MongoDB aggregation converts FP keys to lowercase
		assert.Equal(t, strings.ToLower(fpPk), fpStats[0].FpBtcPkHex)
		assert.Equal(t, uint64(100000), fpStats[0].ActiveTvl)
		assert.Equal(t, uint64(1), fpStats[0].ActiveDelegations)
	})

	t.Run("CalculateActiveStatsAggregated - multiple active delegations with different FPs", func(t *testing.T) {
		resetDatabase(t)

		fpPk1 := randomBTCpk(t)
		fpPk2 := randomBTCpk(t)

		// Delegation 1 to FP1
		delegation1 := createDelegation(t)
		delegation1.State = types.StateActive
		delegation1.StakingAmount = 100000
		delegation1.FinalityProviderBtcPksHex = []string{fpPk1}
		err := testDB.SaveNewBTCDelegation(ctx, delegation1)
		require.NoError(t, err)

		// Delegation 2 to FP2
		delegation2 := createDelegation(t)
		delegation2.State = types.StateActive
		delegation2.StakingAmount = 200000
		delegation2.FinalityProviderBtcPksHex = []string{fpPk2}
		err = testDB.SaveNewBTCDelegation(ctx, delegation2)
		require.NoError(t, err)

		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(300000), tvl)
		assert.Equal(t, uint64(2), delegations)
		require.Len(t, fpStats, 2)

		// Check FP stats (order may vary)
		// Keys are returned in lowercase
		fpStatsMap := make(map[string]uint64)
		for _, stat := range fpStats {
			fpStatsMap[stat.FpBtcPkHex] = stat.ActiveTvl
		}
		assert.Equal(t, uint64(100000), fpStatsMap[strings.ToLower(fpPk1)])
		assert.Equal(t, uint64(200000), fpStatsMap[strings.ToLower(fpPk2)])
	})

	t.Run("CalculateActiveStatsAggregated - multiple delegations to same FP", func(t *testing.T) {
		resetDatabase(t)

		fpPk := randomBTCpk(t)

		// Delegation 1 to same FP
		delegation1 := createDelegation(t)
		delegation1.State = types.StateActive
		delegation1.StakingAmount = 100000
		delegation1.FinalityProviderBtcPksHex = []string{fpPk}
		err := testDB.SaveNewBTCDelegation(ctx, delegation1)
		require.NoError(t, err)

		// Delegation 2 to same FP
		delegation2 := createDelegation(t)
		delegation2.State = types.StateActive
		delegation2.StakingAmount = 150000
		delegation2.FinalityProviderBtcPksHex = []string{fpPk}
		err = testDB.SaveNewBTCDelegation(ctx, delegation2)
		require.NoError(t, err)

		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(250000), tvl)
		assert.Equal(t, uint64(2), delegations)
		require.Len(t, fpStats, 1)
		assert.Equal(t, strings.ToLower(fpPk), fpStats[0].FpBtcPkHex)
		assert.Equal(t, uint64(250000), fpStats[0].ActiveTvl)
		assert.Equal(t, uint64(2), fpStats[0].ActiveDelegations)
	})

	t.Run("CalculateActiveStatsAggregated - mixed states", func(t *testing.T) {
		resetDatabase(t)

		fpPk := randomBTCpk(t)

		// Active delegation
		delegation1 := createDelegation(t)
		delegation1.State = types.StateActive
		delegation1.StakingAmount = 100000
		delegation1.FinalityProviderBtcPksHex = []string{fpPk}
		err := testDB.SaveNewBTCDelegation(ctx, delegation1)
		require.NoError(t, err)

		// Pending delegation (should not be counted)
		delegation2 := createDelegation(t)
		delegation2.State = types.StatePending
		delegation2.StakingAmount = 50000
		delegation2.FinalityProviderBtcPksHex = []string{fpPk}
		err = testDB.SaveNewBTCDelegation(ctx, delegation2)
		require.NoError(t, err)

		// Withdrawn delegation (should not be counted)
		delegation3 := createDelegation(t)
		delegation3.State = types.StateWithdrawn
		delegation3.StakingAmount = 75000
		delegation3.FinalityProviderBtcPksHex = []string{fpPk}
		err = testDB.SaveNewBTCDelegation(ctx, delegation3)
		require.NoError(t, err)

		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		// Only the active delegation should be counted
		assert.Equal(t, uint64(100000), tvl)
		assert.Equal(t, uint64(1), delegations)
		require.Len(t, fpStats, 1)
		assert.Equal(t, uint64(100000), fpStats[0].ActiveTvl)
		assert.Equal(t, uint64(1), fpStats[0].ActiveDelegations)
	})

	t.Run("UpsertOverallStats", func(t *testing.T) {
		// First insert
		err := testDB.UpsertOverallStats(ctx, 1000000, 10)
		require.NoError(t, err)

		// Update with new values
		err = testDB.UpsertOverallStats(ctx, 2000000, 20)
		require.NoError(t, err)

		// Note: We can't directly query the stats collection in this test
		// because it's not exposed through the DbInterface
		// The important thing is that it doesn't error
	})

	t.Run("UpsertFinalityProviderStats - creates stats document", func(t *testing.T) {
		resetDatabase(t)

		fpPk := randomBTCpk(t)

		// Update stats - should create new document in stats collection
		err := testDB.UpsertFinalityProviderStats(ctx, fpPk, 500000, 5)
		require.NoError(t, err)

		// Update again with different values - should update existing document
		err = testDB.UpsertFinalityProviderStats(ctx, fpPk, 750000, 8)
		require.NoError(t, err)

		// We can't directly query the stats collection through the interface,
		// but the important thing is that it doesn't error and uses upsert
		// The actual data integrity is tested by the aggregation tests
	})

	t.Run("UpsertFinalityProviderStats - multiple FPs", func(t *testing.T) {
		resetDatabase(t)

		fpPk1 := randomBTCpk(t)
		fpPk2 := randomBTCpk(t)

		// Create stats for multiple FPs
		err := testDB.UpsertFinalityProviderStats(ctx, fpPk1, 500000, 5)
		require.NoError(t, err)

		err = testDB.UpsertFinalityProviderStats(ctx, fpPk2, 300000, 3)
		require.NoError(t, err)

		// Both should succeed without errors
	})

	t.Run("CalculateActiveStatsAggregated - case insensitive FP keys", func(t *testing.T) {
		resetDatabase(t)

		// Create delegations with FP keys in different cases
		fpPkUpper := "ABCDEF123456"
		fpPkLower := "abcdef123456"

		delegation1 := createDelegation(t)
		delegation1.State = types.StateActive
		delegation1.StakingAmount = 100000
		delegation1.FinalityProviderBtcPksHex = []string{fpPkUpper}
		err := testDB.SaveNewBTCDelegation(ctx, delegation1)
		require.NoError(t, err)

		delegation2 := createDelegation(t)
		delegation2.State = types.StateActive
		delegation2.StakingAmount = 200000
		delegation2.FinalityProviderBtcPksHex = []string{fpPkLower}
		err = testDB.SaveNewBTCDelegation(ctx, delegation2)
		require.NoError(t, err)

		tvl, delegations, fpStats, err := testDB.CalculateActiveStatsAggregated(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(300000), tvl)
		assert.Equal(t, uint64(2), delegations)
		// The aggregation should treat them as the same FP (converted to lowercase)
		require.Len(t, fpStats, 1)
		assert.Equal(t, fpPkLower, fpStats[0].FpBtcPkHex)
		assert.Equal(t, uint64(300000), fpStats[0].ActiveTvl)
		assert.Equal(t, uint64(2), fpStats[0].ActiveDelegations)
	})
}
