package types

import (
	"errors"
	"fmt"
	"strings"

	abcitypes "github.com/cometbft/cometbft/abci/types"
)

// ErrNotAllowlistEvent indicates the event is not an allowlist-related wasm event.
var ErrNotAllowlistEvent = errors.New("not an allowlist event")

const (
	ActionInstantiate         = "instantiate"
	ActionAddToAllowlist      = "add_to_allowlist"
	ActionRemoveFromAllowlist = "remove_from_allowlist"
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

	switch eventType {
	case EventWasm:
		if allowlistEvent.Action != ActionInstantiate || len(allowlistEvent.AllowList) == 0 {
			return nil, ErrNotAllowlistEvent
		}
	case EventWasmAddToAllowlist:
		if len(allowlistEvent.FpPubkeys) == 0 {
			return nil, fmt.Errorf("missing fp_pubkeys in %s event", ActionAddToAllowlist)
		}
	case EventWasmRemoveFromAllowlist:
		if len(allowlistEvent.FpPubkeys) == 0 {
			return nil, fmt.Errorf("missing fp_pubkeys in %s event", ActionRemoveFromAllowlist)
		}
	default:
		return nil, ErrNotAllowlistEvent
	}

	// Validate required fields
	if allowlistEvent.Address == "" {
		return nil, fmt.Errorf("missing address in allowlist event")
	}

	return allowlistEvent, nil
}

// IsInstantiateEvent checks if this is a contract instantiation event
func (e *AllowlistEvent) IsInstantiateEvent() bool {
	return e.EventType == EventWasm && e.Action == ActionInstantiate
}

// IsAddEvent checks if this is an add to allowlist event
func (e *AllowlistEvent) IsAddEvent() bool {
	return e.EventType == EventWasmAddToAllowlist
}

// IsRemoveEvent checks if this is a remove from allowlist event
func (e *AllowlistEvent) IsRemoveEvent() bool {
	return e.EventType == EventWasmRemoveFromAllowlist
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
