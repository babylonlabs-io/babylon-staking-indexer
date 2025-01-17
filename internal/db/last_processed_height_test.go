//go:build integration

package db_test

import (
	"testing"
	"context"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
)

func TestLastProcessedHeight(t *testing.T) {
	ctx := context.Background() // todo (Kirill) change to t.Ctx() after go1.24
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
