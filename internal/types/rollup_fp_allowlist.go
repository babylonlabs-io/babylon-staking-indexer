package types

import (
	"fmt"
	"strings"

	abcitypes "github.com/cometbft/cometbft/abci/types"
)

// Allowlist-related event types for CosmWasm contract events
const (
	// Contract instantiation events
	EventWasmInstantiate EventType = "instantiate"
	EventWasm            EventType = "wasm"

	// Allowlist mutation events
	EventWasmAddToAllowlist      EventType = "wasm-add_to_allowlist"
	EventWasmRemoveFromAllowlist EventType = "wasm-remove_from_allowlist"
)

// AllowlistEvent represents a parsed allowlist-related event
type AllowlistEvent struct {
	EventType  EventType `json:"event_type"`
	Address    string    `json:"address"`
	Action     string    `json:"action,omitempty"`
	FpPubkeys  []string  `json:"fp_pubkeys,omitempty"`
	AllowList  []string  `json:"allow_list,omitempty"`
	NumAdded   string    `json:"num_added,omitempty"`
	NumRemoved string    `json:"num_removed,omitempty"`
	MsgIndex   string    `json:"msg_index,omitempty"`
}

// ParseAllowlistFromString parses a comma-separated string of BTC public keys
func ParseAllowlistFromString(allowlistStr string) []string {
	if allowlistStr == "" {
		return []string{}
	}

	// Split by comma and trim whitespace
	pubkeys := strings.Split(allowlistStr, ",")
	result := make([]string, 0, len(pubkeys))

	for _, pubkey := range pubkeys {
		trimmed := strings.TrimSpace(pubkey)
		if trimmed != "" {
			// Normalize to lowercase for consistent storage
			result = append(result, strings.ToLower(trimmed))
		}
	}

	return result
}

// ParseAllowlistEvent parses ABCI events into AllowlistEvent structs
func ParseAllowlistEvent(event abcitypes.Event) (*AllowlistEvent, error) {
	eventType := EventType(event.Type)

	// Only process allowlist-related events
	if !IsAllowlistEvent(eventType) {
		return nil, fmt.Errorf("not an allowlist event: %s", eventType)
	}

	allowlistEvent := &AllowlistEvent{
		EventType: eventType,
	}

	// Parse attributes
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "_contract_address":
			allowlistEvent.Address = attr.Value
		case "action":
			allowlistEvent.Action = attr.Value
		case "fp_pubkeys":
			allowlistEvent.FpPubkeys = ParseAllowlistFromString(attr.Value)
		case "allow-list":
			allowlistEvent.AllowList = ParseAllowlistFromString(attr.Value)
		case "num_added":
			allowlistEvent.NumAdded = attr.Value
		case "num_removed":
			allowlistEvent.NumRemoved = attr.Value
		case "msg_index":
			allowlistEvent.MsgIndex = attr.Value
		}
	}

	// Validate required fields
	if allowlistEvent.Address == "" {
		return nil, fmt.Errorf("missing address in allowlist event")
	}

	return allowlistEvent, nil
}

// IsAllowlistEvent checks if the event type is allowlist-related
func IsAllowlistEvent(eventType EventType) bool {
	switch eventType {
	case EventWasmInstantiate, EventWasm, EventWasmAddToAllowlist, EventWasmRemoveFromAllowlist:
		return true
	default:
		return false
	}
}

// IsInstantiateEvent checks if this is a contract instantiation event
func (e *AllowlistEvent) IsInstantiateEvent() bool {
	return (e.EventType == EventWasm && e.Action == "instantiate") ||
		e.EventType == EventWasmInstantiate
}

// IsAddEvent checks if this is an add to allowlist event
func (e *AllowlistEvent) IsAddEvent() bool {
	return e.EventType == EventWasmAddToAllowlist ||
		(e.EventType == EventWasm && e.Action == "add_to_allowlist")
}

// IsRemoveEvent checks if this is a remove from allowlist event
func (e *AllowlistEvent) IsRemoveEvent() bool {
	return e.EventType == EventWasmRemoveFromAllowlist ||
		(e.EventType == EventWasm && e.Action == "remove_from_allowlist")
}

// GetPubkeys returns the relevant public keys for this event
func (e *AllowlistEvent) GetPubkeys() []string {
	if len(e.FpPubkeys) > 0 {
		return e.FpPubkeys
	}
	if len(e.AllowList) > 0 {
		return e.AllowList
	}
	return []string{}
}
