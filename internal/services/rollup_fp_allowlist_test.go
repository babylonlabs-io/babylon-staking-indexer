package services

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecializedAllowlistParsersIntegration(t *testing.T) {
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

		allowlistEvent, err := types.ParseInstantiateAllowlistEvent(event)
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

		allowlistEvent, err := types.ParseAddToAllowlistEvent(event)
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

		allowlistEvent, err := types.ParseRemoveFromAllowlistEvent(event)
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

		// Test that all specialized parsers reject invalid event types
		_, err1 := types.ParseInstantiateAllowlistEvent(event)
		assert.Error(t, err1)
		assert.Equal(t, types.ErrNotAllowlistEvent, err1)

		_, err2 := types.ParseAddToAllowlistEvent(event)
		assert.Error(t, err2)
		assert.Equal(t, types.ErrNotAllowlistEvent, err2)

		_, err3 := types.ParseRemoveFromAllowlistEvent(event)
		assert.Error(t, err3)
		assert.Equal(t, types.ErrNotAllowlistEvent, err3)
	})

	t.Run("missing contract address", func(t *testing.T) {
		event := abcitypes.Event{
			Type: "wasm",
			Attributes: []abcitypes.EventAttribute{
				{Key: "action", Value: "instantiate"},
				{Key: "allow-list", Value: "pubkey1"},
			},
		}

		allowlistEvent, err := types.ParseInstantiateAllowlistEvent(event)
		assert.Error(t, err)
		assert.Nil(t, allowlistEvent)
		assert.Equal(t, types.ErrNotAllowlistEvent, err)
	})
}

func TestConstantsUsage(t *testing.T) {
	t.Run("verify constants are defined and used", func(t *testing.T) {
		// Test that constants are defined
		assert.Equal(t, "instantiate", types.ActionInstantiate)
		assert.Equal(t, "add_to_allowlist", types.ActionAddToAllowlist)
		assert.Equal(t, "remove_from_allowlist", types.ActionRemoveFromAllowlist)
	})
}
