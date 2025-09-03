package services

import (
	"context"
	"errors"
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/tests/mocks"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
)

type testServiceDeps struct {
	DB            *mocks.DbInterface
	BTC           *mocks.BtcInterface
	BTCNotifier   *mocks.BtcNotifier
	BBN           *mocks.BbnInterface
	EventConsumer *mocks.EventConsumer
}

func setupTestService(t *testing.T) (*Service, *testServiceDeps) {
	cfg := &config.Config{}
	deps := &testServiceDeps{
		DB:            mocks.NewDbInterface(t),
		BTC:           mocks.NewBtcInterface(t),
		BTCNotifier:   mocks.NewBtcNotifier(t),
		BBN:           mocks.NewBbnInterface(t),
		EventConsumer: mocks.NewEventConsumer(t),
	}

	s := NewService(cfg, deps.DB, deps.BTC, deps.BTCNotifier, deps.BBN, deps.EventConsumer)
	return s, deps
}

// Helper function to create a test BSN
func createTestBSN() *model.BSN {
	return &model.BSN{
		ID:   "test-bsn-id",
		Name: "Test BSN",
		RollupMetadata: &model.ETHL2Metadata{
			Allowlist: []string{"existing_pubkey_1", "existing_pubkey_2"},
		},
	}
}

func TestProcessInstantiateAllowlistEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("successful instantiate with new allowlist", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "test-contract-addr",
			Action:    "instantiate",
			AllowList: []string{"new_pubkey_1", "new_pubkey_2", "new_pubkey_3"},
			MsgIndex:  "0",
		}

		testBSN := createTestBSN()

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", []string{"new_pubkey_1", "new_pubkey_2", "new_pubkey_3"}).Return(nil)

		// Execute
		err := s.processInstantiateAllowlistEvent(ctx, allowlistEvent, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("empty allowlist skips processing", func(t *testing.T) {
		// Setup
		s, _ := setupTestService(t)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "test-contract-addr",
			Action:    "instantiate",
			AllowList: []string{}, // Empty allowlist
			MsgIndex:  "0",
		}

		// Execute
		err := s.processInstantiateAllowlistEvent(ctx, allowlistEvent, 123)

		// Assert - should not error and should not call database
		assert.NoError(t, err)
	})

	t.Run("BSN not found error", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "nonexistent-contract-addr",
			Action:    "instantiate",
			AllowList: []string{"new_pubkey_1"},
			MsgIndex:  "0",
		}

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "nonexistent-contract-addr").Return(nil, errors.New("BSN not found"))

		// Execute
		err := s.processInstantiateAllowlistEvent(ctx, allowlistEvent, 123)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BSN not found for instantiate event")
	})

	t.Run("database update error", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "test-contract-addr",
			Action:    "instantiate",
			AllowList: []string{"new_pubkey_1"},
			MsgIndex:  "0",
		}

		testBSN := createTestBSN()

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", []string{"new_pubkey_1"}).Return(errors.New("database error"))

		// Execute
		err := s.processInstantiateAllowlistEvent(ctx, allowlistEvent, 123)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update BSN allowlist for instantiate")
	})
}

func TestProcessAddAllowlistEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("successful add to existing allowlist", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "new_pubkey_1,new_pubkey_2"},
				{Key: "num_added", Value: "2"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := createTestBSN() // Has existing_pubkey_1, existing_pubkey_2
		expectedNewAllowlist := []string{"existing_pubkey_1", "existing_pubkey_2", "new_pubkey_1", "new_pubkey_2"}

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processAddAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("add to empty allowlist", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "first_pubkey,second_pubkey"},
				{Key: "num_added", Value: "2"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := &model.BSN{
			ID:             "test-bsn-id",
			Name:           "Test BSN",
			RollupMetadata: nil, // Empty allowlist
		}
		expectedNewAllowlist := []string{"first_pubkey", "second_pubkey"}

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processAddAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("add duplicate pubkeys (should not duplicate)", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "existing_pubkey_1,new_unique_pubkey"},
				{Key: "num_added", Value: "1"}, // Only 1 should be added (duplicate ignored)
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := createTestBSN() // Has existing_pubkey_1, existing_pubkey_2
		expectedNewAllowlist := []string{"existing_pubkey_1", "existing_pubkey_2", "new_unique_pubkey"}

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processAddAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("event with empty pubkeys fails parsing", func(t *testing.T) {
		// Setup
		s, _ := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: ""}, // Empty - parsing will fail
				{Key: "msg_index", Value: "0"},
			},
		}

		// Execute
		err := s.processAddAllowlistEvent(ctx, event, 123)

		// Assert - should error because parsing fails
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse add to allowlist event")
	})

	t.Run("parsing error", func(t *testing.T) {
		// Setup
		s, _ := setupTestService(t)

		event := abcitypes.Event{
			Type: "invalid-event-type", // Will cause parsing to fail
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
			},
		}

		// Execute
		err := s.processAddAllowlistEvent(ctx, event, 123)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse add to allowlist event")
	})
}

func TestProcessRemoveAllowlistEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("successful remove from allowlist", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "existing_pubkey_1"},
				{Key: "num_removed", Value: "1"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := createTestBSN()                            // Has existing_pubkey_1, existing_pubkey_2
		expectedNewAllowlist := []string{"existing_pubkey_2"} // Only existing_pubkey_2 should remain

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processRemoveAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("remove multiple pubkeys", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "existing_pubkey_1,existing_pubkey_2"},
				{Key: "num_removed", Value: "2"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := createTestBSN()         // Has existing_pubkey_1, existing_pubkey_2
		expectedNewAllowlist := []string{} // All should be removed

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processRemoveAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("remove non-existent pubkey (should not error)", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "non_existent_pubkey"},
				{Key: "num_removed", Value: "0"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := createTestBSN()                                                 // Has existing_pubkey_1, existing_pubkey_2
		expectedNewAllowlist := []string{"existing_pubkey_1", "existing_pubkey_2"} // Should remain unchanged

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processRemoveAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("remove from empty allowlist", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "some_pubkey"},
				{Key: "num_removed", Value: "0"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := &model.BSN{
			ID:             "test-bsn-id",
			Name:           "Test BSN",
			RollupMetadata: nil, // Empty allowlist
		}
		expectedNewAllowlist := []string{} // Should remain empty

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		deps.DB.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := s.processRemoveAllowlistEvent(ctx, event, 123)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("event with empty pubkeys fails parsing", func(t *testing.T) {
		// Setup
		s, _ := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: ""}, // Empty - parsing will fail
				{Key: "msg_index", Value: "0"},
			},
		}

		// Execute
		err := s.processRemoveAllowlistEvent(ctx, event, 123)

		// Assert - should error because parsing fails
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse remove from allowlist event")
	})

	t.Run("BSN not found error", func(t *testing.T) {
		// Setup
		s, deps := setupTestService(t)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "nonexistent-contract-addr"},
				{Key: "fp_pubkeys", Value: "some_pubkey"},
				{Key: "msg_index", Value: "0"},
			},
		}

		// Mock expectations
		deps.DB.On("GetBSNByAddress", ctx, "nonexistent-contract-addr").Return(nil, errors.New("BSN not found"))

		// Execute
		err := s.processRemoveAllowlistEvent(ctx, event, 123)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BSN not found for remove from allowlist event")
	})
}
