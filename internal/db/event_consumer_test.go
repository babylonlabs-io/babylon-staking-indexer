//go:build integration

package db_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventConsumer(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	t.Run("save", func(t *testing.T) {
		doc := &model.BSN{
			ID:   "event-id",
			Name: "some name",
		}

		err := testDB.SaveBSN(ctx, doc)
		require.NoError(t, err)

		fetchedDoc, err := testDB.GetBSN(ctx, doc.ID)
		require.NoError(t, err)
		assert.Equal(t, doc, fetchedDoc)

		err = testDB.SaveBSN(ctx, doc)
		assert.True(t, db.IsDuplicateKeyError(err))
	})
	t.Run("get", func(t *testing.T) {
		doc, err := testDB.GetBSN(ctx, "non-existing-id")
		assert.True(t, db.IsNotFoundError(err))
		assert.Nil(t, doc)
	})
}
