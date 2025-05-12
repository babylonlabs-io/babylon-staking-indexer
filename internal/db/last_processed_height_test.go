//go:build integration

package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastProcessedHeight(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})
	t.Run("no documents", func(t *testing.T) {
		height, err := testDB.GetLastProcessedBbnHeight(ctx)
		require.NoError(t, err)
		assert.Zero(t, height)
	})
	t.Run("upsert", func(t *testing.T) {
		const (
			initialHeight = 100
			updatedHeight = 1000
		)

		// on first iteration we insert doc with initialHeight
		// on second we update the doc with updatedHeight
		for _, height := range []uint64{initialHeight, updatedHeight} {
			err := testDB.UpdateLastProcessedBbnHeight(ctx, height)
			require.NoError(t, err)

			actualHeight, err := testDB.GetLastProcessedBbnHeight(ctx)
			require.NoError(t, err)
			assert.Equal(t, height, actualHeight)
		}
	})
}
