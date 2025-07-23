//go:build integration

package db_test

import (
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNetworkInfo(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	t.Run("not found", func(t *testing.T) {
		doc, err := testDB.GetNetworkInfo(ctx)
		assert.True(t, db.IsNotFoundError(err))
		assert.Nil(t, doc)
	})
	t.Run("ok", func(t *testing.T) {
		ids := []string{"chain-id-1", "chain-id-2"}

		for _, id := range ids {
			doc := &model.NetworkInfo{
				ChainID: id,
			}
			err := testDB.UpsertNetworkInfo(ctx, doc)
			require.NoError(t, err)

			foundDoc, err := testDB.GetNetworkInfo(ctx)
			require.NoError(t, err)
			assert.Equal(t, doc, foundDoc)
		}
	})
}
