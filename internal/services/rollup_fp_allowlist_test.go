package services

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeAllowlistChanges(t *testing.T) {
	srv := &Service{}

	t.Run("instantiate event - full snapshot", func(t *testing.T) {
		bsn := &model.BSN{
			RollupMetadata: &model.ETHL2Metadata{
				Allowlist: []string{"old_pubkey1", "old_pubkey2"},
			},
		}
		pubkeys := []string{"new_pubkey1", "old_pubkey1", "new_pubkey2"}

		added, removed := srv.computeAllowlistChanges(bsn, pubkeys, types.ActionInstantiate)

		assert.ElementsMatch(t, []string{"new_pubkey1", "new_pubkey2"}, added)
		assert.ElementsMatch(t, []string{"old_pubkey2"}, removed)
	})

	t.Run("add to allowlist event", func(t *testing.T) {
		bsn := &model.BSN{}
		pubkeys := []string{"new_pubkey1", "new_pubkey2"}

		added, removed := srv.computeAllowlistChanges(bsn, pubkeys, types.ActionAddToAllowlist)

		assert.ElementsMatch(t, []string{"new_pubkey1", "new_pubkey2"}, added)
		assert.Nil(t, removed)
	})

	t.Run("remove from allowlist event", func(t *testing.T) {
		bsn := &model.BSN{}
		pubkeys := []string{"remove_pubkey1", "remove_pubkey2"}

		added, removed := srv.computeAllowlistChanges(bsn, pubkeys, types.ActionRemoveFromAllowlist)

		assert.Nil(t, added)
		assert.ElementsMatch(t, []string{"remove_pubkey1", "remove_pubkey2"}, removed)
	})

	t.Run("unknown event type", func(t *testing.T) {
		bsn := &model.BSN{}
		pubkeys := []string{"some_pubkey"}

		added, removed := srv.computeAllowlistChanges(bsn, pubkeys, "unknown")

		assert.Nil(t, added)
		assert.Nil(t, removed)
	})

	t.Run("instantiate with empty old allowlist", func(t *testing.T) {
		bsn := &model.BSN{
			RollupMetadata: &model.ETHL2Metadata{},
		}
		pubkeys := []string{"new_pubkey1", "new_pubkey2"}

		added, removed := srv.computeAllowlistChanges(bsn, pubkeys, types.ActionInstantiate)

		assert.ElementsMatch(t, []string{"new_pubkey1", "new_pubkey2"}, added)
		assert.Empty(t, removed) // Empty slice, not nil
	})

	t.Run("instantiate with no rollup metadata", func(t *testing.T) {
		bsn := &model.BSN{}
		pubkeys := []string{"new_pubkey1", "new_pubkey2"}

		added, removed := srv.computeAllowlistChanges(bsn, pubkeys, types.ActionInstantiate)

		assert.ElementsMatch(t, []string{"new_pubkey1", "new_pubkey2"}, added)
		assert.Empty(t, removed) // Empty slice, not nil
	})
}

func TestNormalizePubkeys(t *testing.T) {
	t.Run("normalize and deduplicate pubkeys", func(t *testing.T) {
		pubkeys := []string{"PUBKEY1", "pubkey2", "", "PUBKEY1", "pubkey3", "  pubkey4  "}

		result := normalizePubkeys(pubkeys)

		expected := []string{"pubkey1", "pubkey2", "pubkey3", "  pubkey4  "} // whitespace is preserved
		assert.ElementsMatch(t, expected, result)
	})

	t.Run("empty input", func(t *testing.T) {
		pubkeys := []string{}

		result := normalizePubkeys(pubkeys)

		assert.Empty(t, result)
	})

	t.Run("only empty strings", func(t *testing.T) {
		pubkeys := []string{"", "  ", ""}

		result := normalizePubkeys(pubkeys)

		expected := []string{"  "} // non-empty strings are kept
		assert.ElementsMatch(t, expected, result)
	})

	t.Run("mixed case with duplicates", func(t *testing.T) {
		pubkeys := []string{"ABC", "def", "ABC", "DEF", "ghi"}

		result := normalizePubkeys(pubkeys)

		expected := []string{"abc", "def", "ghi"}
		assert.ElementsMatch(t, expected, result)
	})

	t.Run("whitespace handling", func(t *testing.T) {
		pubkeys := []string{"  abc  ", "\tdef\t", " ghi ", "abc"}

		result := normalizePubkeys(pubkeys)

		expected := []string{"  abc  ", "\tdef\t", " ghi ", "abc"} // whitespace is preserved
		assert.ElementsMatch(t, expected, result)
	})
}

func TestParseAllowlistEventIntegration(t *testing.T) {
	t.Run("instantiate event parsing", func(t *testing.T) {
		event := abcitypes.Event{
			Type: "wasm",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test_address"},
				{Key: "action", Value: "instantiate"},
				{Key: "allow-list", Value: "pubkey1,pubkey2,pubkey3"},
				{Key: "msg_index", Value: "0"},
			},
		}

		allowlistEvent, err := types.ParseAllowlistEvent(event)
		require.NoError(t, err)
		require.NotNil(t, allowlistEvent)

		assert.Equal(t, "test_address", allowlistEvent.Address)
		assert.Equal(t, []string{"pubkey1", "pubkey2", "pubkey3"}, allowlistEvent.AllowList)
		assert.True(t, allowlistEvent.IsInstantiateEvent())
		assert.Equal(t, []string{"pubkey1", "pubkey2", "pubkey3"}, allowlistEvent.GetPubkeys())
	})

	t.Run("add to allowlist event parsing", func(t *testing.T) {
		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test_address"},
				{Key: "fp_pubkeys", Value: "new_pubkey1,new_pubkey2"},
				{Key: "msg_index", Value: "0"},
			},
		}

		allowlistEvent, err := types.ParseAllowlistEvent(event)
		require.NoError(t, err)
		require.NotNil(t, allowlistEvent)

		assert.Equal(t, "test_address", allowlistEvent.Address)
		assert.Equal(t, []string{"new_pubkey1", "new_pubkey2"}, allowlistEvent.FpPubkeys)
		assert.True(t, allowlistEvent.IsAddEvent())
		assert.Equal(t, []string{"new_pubkey1", "new_pubkey2"}, allowlistEvent.GetPubkeys())
	})

	t.Run("remove from allowlist event parsing", func(t *testing.T) {
		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test_address"},
				{Key: "fp_pubkeys", Value: "remove_pubkey1,remove_pubkey2"},
				{Key: "msg_index", Value: "0"},
			},
		}

		allowlistEvent, err := types.ParseAllowlistEvent(event)
		require.NoError(t, err)
		require.NotNil(t, allowlistEvent)

		assert.Equal(t, "test_address", allowlistEvent.Address)
		assert.Equal(t, []string{"remove_pubkey1", "remove_pubkey2"}, allowlistEvent.FpPubkeys)
		assert.True(t, allowlistEvent.IsRemoveEvent())
		assert.Equal(t, []string{"remove_pubkey1", "remove_pubkey2"}, allowlistEvent.GetPubkeys())
	})

	t.Run("invalid event type", func(t *testing.T) {
		event := abcitypes.Event{
			Type: "invalid_event_type",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test_address"},
			},
		}

		allowlistEvent, err := types.ParseAllowlistEvent(event)
		assert.Error(t, err)
		assert.Nil(t, allowlistEvent)
		assert.Equal(t, types.ErrNotAllowlistEvent, err)
	})

	t.Run("missing contract address", func(t *testing.T) {
		event := abcitypes.Event{
			Type: "wasm",
			Attributes: []abcitypes.EventAttribute{
				{Key: "action", Value: "instantiate"},
				{Key: "allow-list", Value: "pubkey1"},
			},
		}

		allowlistEvent, err := types.ParseAllowlistEvent(event)
		assert.Error(t, err)
		assert.Nil(t, allowlistEvent)
		assert.Contains(t, err.Error(), "missing address")
	})
}

func TestConstantsUsage(t *testing.T) {
	t.Run("verify constants are defined and used", func(t *testing.T) {
		// Test that constants are defined
		assert.Equal(t, "instantiate", types.ActionInstantiate)
		assert.Equal(t, "add_to_allowlist", types.ActionAddToAllowlist)
		assert.Equal(t, "remove_from_allowlist", types.ActionRemoveFromAllowlist)

		// Test that they work with our functions
		srv := &Service{}
		bsn := &model.BSN{}

		// Test each constant works with computeAllowlistChanges
		added, removed := srv.computeAllowlistChanges(bsn, []string{"test"}, types.ActionInstantiate)
		assert.ElementsMatch(t, []string{"test"}, added)
		assert.Empty(t, removed) // Empty slice, not nil

		added, removed = srv.computeAllowlistChanges(bsn, []string{"test"}, types.ActionAddToAllowlist)
		assert.ElementsMatch(t, []string{"test"}, added)
		assert.Nil(t, removed)

		added, removed = srv.computeAllowlistChanges(bsn, []string{"test"}, types.ActionRemoveFromAllowlist)
		assert.Nil(t, added)
		assert.ElementsMatch(t, []string{"test"}, removed)
	})
}
