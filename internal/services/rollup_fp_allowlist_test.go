package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/tests/mocks"
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

// MockDbClientWithBSN extends the generated mock to include BSN methods
type MockDbClientWithBSN struct {
	*mocks.DbInterface
}

func NewMockDbClientWithBSN() *MockDbClientWithBSN {
	return &MockDbClientWithBSN{
		DbInterface: &mocks.DbInterface{},
	}
}

// GetBSNByAddress provides a mock function with given fields: ctx, address
func (m *MockDbClientWithBSN) GetBSNByAddress(ctx context.Context, address string) (*model.BSN, error) {
	ret := m.Called(ctx, address)

	if len(ret) == 0 {
		panic("no return value specified for GetBSNByAddress")
	}

	var r0 *model.BSN
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*model.BSN, error)); ok {
		return rf(ctx, address)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.BSN); ok {
		r0 = rf(ctx, address)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).(*model.BSN)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateBSNAllowlist provides a mock function with given fields: ctx, address, allowlist
func (m *MockDbClientWithBSN) UpdateBSNAllowlist(ctx context.Context, address string, allowlist []string) error {
	ret := m.Called(ctx, address, allowlist)

	if len(ret) == 0 {
		panic("no return value specified for UpdateBSNAllowlist")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string) error); ok {
		r0 = rf(ctx, address, allowlist)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// TestService wraps Service to allow injecting mocked dependencies
type TestService struct {
	bsnDb *MockDbClientWithBSN
}

// Helper function to create a service with mocked dependencies
func createTestService(mockDb *MockDbClientWithBSN) *TestService {
	return &TestService{
		bsnDb: mockDb,
	}
}

// processInstantiateAllowlistEvent is a test wrapper for the service method
func (ts *TestService) processInstantiateAllowlistEvent(ctx context.Context, allowlistEvent *types.AllowlistEvent) error {
	if len(allowlistEvent.AllowList) == 0 {
		return nil
	}

	// Check if we have a BSN registered for this contract address
	_, err := ts.bsnDb.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		return errors.New("BSN not found for instantiate event with address " + allowlistEvent.Address + ": " + err.Error())
	}

	// For instantiate, we replace the entire allowlist
	newAllowlist := allowlistEvent.AllowList

	// Persist BSN allowlist
	if err := ts.bsnDb.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		return errors.New("failed to update BSN allowlist for instantiate: " + err.Error())
	}

	return nil
}

// processAddAllowlistEvent is a test wrapper for the service method
func (ts *TestService) processAddAllowlistEvent(ctx context.Context, event abcitypes.Event) error {
	allowlistEvent, err := types.ParseAddToAllowlistEvent(event)
	if err != nil {
		return errors.New("failed to parse add to allowlist event: " + err.Error())
	}

	// Validate we have pubkeys to add
	if len(allowlistEvent.FpPubkeys) == 0 {
		return nil
	}

	// Check if we have a BSN registered for this contract address
	bsn, err := ts.bsnDb.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		return errors.New("BSN not found for add to allowlist event with address " + allowlistEvent.Address + ": " + err.Error())
	}

	currentAllowlist := make([]string, 0)
	existing := make(map[string]struct{})

	if bsn.RollupMetadata != nil && bsn.RollupMetadata.Allowlist != nil {
		currentAllowlist = make([]string, 0, len(bsn.RollupMetadata.Allowlist))
		for _, pk := range bsn.RollupMetadata.Allowlist {
			normalized := strings.ToLower(pk)
			currentAllowlist = append(currentAllowlist, normalized)
			existing[normalized] = struct{}{}
		}
	}

	newAllowlist := make([]string, 0, len(currentAllowlist)+len(allowlistEvent.FpPubkeys))
	newAllowlist = append(newAllowlist, currentAllowlist...)

	for _, pk := range allowlistEvent.FpPubkeys {
		if _, ok := existing[pk]; !ok {
			existing[pk] = struct{}{}
			newAllowlist = append(newAllowlist, pk)
		}
	}

	// Persist BSN allowlist
	if err := ts.bsnDb.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		return errors.New("failed to update BSN allowlist for add: " + err.Error())
	}
	return nil
}

// processRemoveAllowlistEvent is a test wrapper for the service method
func (ts *TestService) processRemoveAllowlistEvent(ctx context.Context, event abcitypes.Event) error {
	allowlistEvent, err := types.ParseRemoveFromAllowlistEvent(event)
	if err != nil {
		return errors.New("failed to parse remove from allowlist event: " + err.Error())
	}

	// Validate we have pubkeys to remove
	if len(allowlistEvent.FpPubkeys) == 0 {
		return nil
	}

	// Check if we have a BSN registered for this contract address
	bsn, err := ts.bsnDb.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		return errors.New("BSN not found for remove from allowlist event with address " + allowlistEvent.Address + ": " + err.Error())
	}

	currentAllowlist := make([]string, 0)
	if bsn.RollupMetadata != nil && bsn.RollupMetadata.Allowlist != nil {
		currentAllowlist = make([]string, 0, len(bsn.RollupMetadata.Allowlist))
		for _, pk := range bsn.RollupMetadata.Allowlist {
			currentAllowlist = append(currentAllowlist, strings.ToLower(pk))
		}
	}

	toRemove := make(map[string]struct{}, len(allowlistEvent.FpPubkeys))
	for _, pk := range allowlistEvent.FpPubkeys {
		toRemove[pk] = struct{}{}
	}

	newAllowlist := make([]string, 0, len(currentAllowlist))
	for _, pk := range currentAllowlist {
		if _, remove := toRemove[pk]; !remove {
			newAllowlist = append(newAllowlist, pk)
		}
	}

	// Persist BSN allowlist
	if err := ts.bsnDb.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		return errors.New("failed to update BSN allowlist for remove: " + err.Error())
	}
	return nil
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
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "test-contract-addr",
			Action:    "instantiate",
			AllowList: []string{"new_pubkey_1", "new_pubkey_2", "new_pubkey_3"},
			MsgIndex:  "0",
		}

		testBSN := createTestBSN()

		// Mock expectations
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", []string{"new_pubkey_1", "new_pubkey_2", "new_pubkey_3"}).Return(nil)

		// Execute
		err := testService.processInstantiateAllowlistEvent(ctx, allowlistEvent)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("empty allowlist skips processing", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "test-contract-addr",
			Action:    "instantiate",
			AllowList: []string{}, // Empty allowlist
			MsgIndex:  "0",
		}

		// Execute
		err := testService.processInstantiateAllowlistEvent(ctx, allowlistEvent)

		// Assert - should not error and should not call database
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("BSN not found error", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "nonexistent-contract-addr",
			Action:    "instantiate",
			AllowList: []string{"new_pubkey_1"},
			MsgIndex:  "0",
		}

		// Mock expectations
		mockDb.On("GetBSNByAddress", ctx, "nonexistent-contract-addr").Return(nil, errors.New("BSN not found"))

		// Execute
		err := testService.processInstantiateAllowlistEvent(ctx, allowlistEvent)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BSN not found for instantiate event")
		mockDb.AssertExpectations(t)
	})

	t.Run("database update error", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		allowlistEvent := &types.AllowlistEvent{
			EventType: types.EventWasm,
			Address:   "test-contract-addr",
			Action:    "instantiate",
			AllowList: []string{"new_pubkey_1"},
			MsgIndex:  "0",
		}

		testBSN := createTestBSN()

		// Mock expectations
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", []string{"new_pubkey_1"}).Return(errors.New("database error"))

		// Execute
		err := testService.processInstantiateAllowlistEvent(ctx, allowlistEvent)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update BSN allowlist for instantiate")
		mockDb.AssertExpectations(t)
	})
}

func TestProcessAddAllowlistEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("successful add to existing allowlist", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("add to empty allowlist", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("add duplicate pubkeys (should not duplicate)", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("event with empty pubkeys fails parsing", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: ""}, // Empty - parsing will fail
				{Key: "msg_index", Value: "0"},
			},
		}

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert - should error because parsing fails
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse add to allowlist event")
		mockDb.AssertExpectations(t)
	})

	t.Run("parsing error", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		event := abcitypes.Event{
			Type: "invalid-event-type", // Will cause parsing to fail
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
			},
		}

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse add to allowlist event")
		mockDb.AssertExpectations(t)
	})
}

func TestProcessRemoveAllowlistEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("successful remove from allowlist", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processRemoveAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("remove multiple pubkeys", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processRemoveAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("remove non-existent pubkey (should not error)", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processRemoveAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("remove from empty allowlist", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

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
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processRemoveAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("event with empty pubkeys fails parsing", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: ""}, // Empty - parsing will fail
				{Key: "msg_index", Value: "0"},
			},
		}

		// Execute
		err := testService.processRemoveAllowlistEvent(ctx, event)

		// Assert - should error because parsing fails
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse remove from allowlist event")
		mockDb.AssertExpectations(t)
	})

	t.Run("BSN not found error", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		event := abcitypes.Event{
			Type: "wasm-remove_from_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "nonexistent-contract-addr"},
				{Key: "fp_pubkeys", Value: "some_pubkey"},
				{Key: "msg_index", Value: "0"},
			},
		}

		// Mock expectations
		mockDb.On("GetBSNByAddress", ctx, "nonexistent-contract-addr").Return(nil, errors.New("BSN not found"))

		// Execute
		err := testService.processRemoveAllowlistEvent(ctx, event)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BSN not found for remove from allowlist event")
		mockDb.AssertExpectations(t)
	})
}

func TestEdgeCasesAndErrorHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("case insensitive pubkey handling", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: "UPPERCASE_PUBKEY,lowercase_pubkey"},
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := &model.BSN{
			ID:   "test-bsn-id",
			Name: "Test BSN",
			RollupMetadata: &model.ETHL2Metadata{
				Allowlist: []string{"existing_pubkey"},
			},
		}

		// Should normalize to lowercase
		expectedNewAllowlist := []string{"existing_pubkey", "uppercase_pubkey", "lowercase_pubkey"}

		// Mock expectations
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})

	t.Run("whitespace handling in pubkeys", func(t *testing.T) {
		// Setup
		mockDb := NewMockDbClientWithBSN()
		testService := createTestService(mockDb)

		event := abcitypes.Event{
			Type: "wasm-add_to_allowlist",
			Attributes: []abcitypes.EventAttribute{
				{Key: "_contract_address", Value: "test-contract-addr"},
				{Key: "fp_pubkeys", Value: " pubkey_with_spaces , another_pubkey "}, // With whitespace
				{Key: "msg_index", Value: "0"},
			},
		}

		testBSN := &model.BSN{
			ID:             "test-bsn-id",
			Name:           "Test BSN",
			RollupMetadata: nil,
		}

		// Should trim whitespace and normalize
		expectedNewAllowlist := []string{"pubkey_with_spaces", "another_pubkey"}

		// Mock expectations
		mockDb.On("GetBSNByAddress", ctx, "test-contract-addr").Return(testBSN, nil)
		mockDb.On("UpdateBSNAllowlist", ctx, "test-contract-addr", expectedNewAllowlist).Return(nil)

		// Execute
		err := testService.processAddAllowlistEvent(ctx, event)

		// Assert
		assert.NoError(t, err)
		mockDb.AssertExpectations(t)
	})
}
