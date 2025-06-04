//go:build integration_v3

package db_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventConsumer(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	t.Run("get", func(t *testing.T) {
		doc, err := testDB.GetEventConsumerByID(ctx, "non-existing-id")
		require.Error(t, err)
		assert.True(t, db.IsNotFoundError(err))
		assert.Nil(t, doc)
	})
}
