package types

import (
	"testing"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAllowlistFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single pubkey",
			input:    "d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf",
			expected: []string{"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf"},
		},
		{
			name:  "multiple pubkeys",
			input: "d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf,eb70add112d8b289231da8dcc448bdadfc8fce9d1a1db113650dbc7aa01fe8c1",
			expected: []string{
				"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf",
				"eb70add112d8b289231da8dcc448bdadfc8fce9d1a1db113650dbc7aa01fe8c1",
			},
		},
		{
			name:  "pubkeys with whitespace",
			input: " d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf , eb70add112d8b289231da8dcc448bdadfc8fce9d1a1db113650dbc7aa01fe8c1 ",
			expected: []string{
				"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf",
				"eb70add112d8b289231da8dcc448bdadfc8fce9d1a1db113650dbc7aa01fe8c1",
			},
		},
		{
			name:     "uppercase pubkeys (should normalize to lowercase)",
			input:    "D87687800CF9E51026A787339D9DE9DAE3E4DBED9ACA7167F0C100F39E8788CF",
			expected: []string{"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAllowlistFromString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAllowlistEvent(t *testing.T) {
	tests := []struct {
		name        string
		event       abcitypes.Event
		expected    *AllowlistEvent
		expectError bool
	}{
		{
			name: "instantiate event with allow-list",
			event: abcitypes.Event{
				Type: "wasm",
				Attributes: []abcitypes.EventAttribute{
					{Key: "_contract_address", Value: "bbn186hnxztn0gh7090rqjuvw8ln6zw08qt4q88jl6ed2tlzhfhq4hpq2n92jj"},
					{Key: "action", Value: "instantiate"},
					{Key: "allow-list", Value: "d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf,eb70add112d8b289231da8dcc448bdadfc8fce9d1a1db113650dbc7aa01fe8c1"},
					{Key: "msg_index", Value: "0"},
				},
			},
			expected: &AllowlistEvent{
				EventType:       EventWasm,
				ContractAddress: "bbn186hnxztn0gh7090rqjuvw8ln6zw08qt4q88jl6ed2tlzhfhq4hpq2n92jj",
				Action:          "instantiate",
				AllowList: []string{
					"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf",
					"eb70add112d8b289231da8dcc448bdadfc8fce9d1a1db113650dbc7aa01fe8c1",
				},
				MsgIndex: "0",
			},
			expectError: false,
		},
		{
			name: "add to allowlist event",
			event: abcitypes.Event{
				Type: "wasm-add_to_allowlist",
				Attributes: []abcitypes.EventAttribute{
					{Key: "_contract_address", Value: "bbn186hnxztn0gh7090rqjuvw8ln6zw08qt4q88jl6ed2tlzhfhq4hpq2n92jj"},
					{Key: "fp_pubkeys", Value: "d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf"},
					{Key: "msg_index", Value: "0"},
				},
			},
			expected: &AllowlistEvent{
				EventType:       EventWasmAddToAllowlist,
				ContractAddress: "bbn186hnxztn0gh7090rqjuvw8ln6zw08qt4q88jl6ed2tlzhfhq4hpq2n92jj",
				FpPubkeys:       []string{"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf"},
				MsgIndex:        "0",
			},
			expectError: false,
		},
		{
			name: "remove from allowlist event",
			event: abcitypes.Event{
				Type: "wasm-remove_from_allowlist",
				Attributes: []abcitypes.EventAttribute{
					{Key: "_contract_address", Value: "bbn186hnxztn0gh7090rqjuvw8ln6zw08qt4q88jl6ed2tlzhfhq4hpq2n92jj"},
					{Key: "fp_pubkeys", Value: "d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf"},
					{Key: "msg_index", Value: "0"},
				},
			},
			expected: &AllowlistEvent{
				EventType:       EventWasmRemoveFromAllowlist,
				ContractAddress: "bbn186hnxztn0gh7090rqjuvw8ln6zw08qt4q88jl6ed2tlzhfhq4hpq2n92jj",
				FpPubkeys:       []string{"d87687800cf9e51026a787339d9de9dae3e4dbed9aca7167f0c100f39e8788cf"},
				MsgIndex:        "0",
			},
			expectError: false,
		},
		{
			name: "non-allowlist event",
			event: abcitypes.Event{
				Type: "transfer",
				Attributes: []abcitypes.EventAttribute{
					{Key: "recipient", Value: "some_address"},
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "allowlist event missing contract address",
			event: abcitypes.Event{
				Type: "wasm",
				Attributes: []abcitypes.EventAttribute{
					{Key: "action", Value: "instantiate"},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAllowlistEvent(tt.event)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAllowlistEventMethods(t *testing.T) {
	tests := []struct {
		name          string
		event         AllowlistEvent
		isInstantiate bool
		isAdd         bool
		isRemove      bool
		pubkeys       []string
	}{
		{
			name: "instantiate event",
			event: AllowlistEvent{
				EventType: EventWasm,
				Action:    "instantiate",
				AllowList: []string{"pubkey1", "pubkey2"},
			},
			isInstantiate: true,
			isAdd:         false,
			isRemove:      false,
			pubkeys:       []string{"pubkey1", "pubkey2"},
		},
		{
			name: "add event",
			event: AllowlistEvent{
				EventType: EventWasmAddToAllowlist,
				FpPubkeys: []string{"pubkey3"},
			},
			isInstantiate: false,
			isAdd:         true,
			isRemove:      false,
			pubkeys:       []string{"pubkey3"},
		},
		{
			name: "remove event",
			event: AllowlistEvent{
				EventType: EventWasmRemoveFromAllowlist,
				FpPubkeys: []string{"pubkey4"},
			},
			isInstantiate: false,
			isAdd:         false,
			isRemove:      true,
			pubkeys:       []string{"pubkey4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isInstantiate, tt.event.IsInstantiateEvent())
			assert.Equal(t, tt.isAdd, tt.event.IsAddEvent())
			assert.Equal(t, tt.isRemove, tt.event.IsRemoveEvent())
			assert.Equal(t, tt.pubkeys, tt.event.GetPubkeys())
		})
	}
}

func TestIsAllowlistEvent(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  bool
	}{
		{EventWasmInstantiate, true},
		{EventWasm, true},
		{EventWasmAddToAllowlist, true},
		{EventWasmRemoveFromAllowlist, true},
		{EventBTCDelegationCreated, false},
		{EventType("transfer"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAllowlistEvent(tt.eventType))
		})
	}
}

