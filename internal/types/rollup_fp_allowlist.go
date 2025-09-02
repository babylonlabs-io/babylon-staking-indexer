package types

import (
	"errors"
	"fmt"
	"strings"

	abcitypes "github.com/cometbft/cometbft/abci/types"
)

// ErrNotAllowlistEvent indicates the event is not an allowlist-related wasm event.
var ErrNotAllowlistEvent = errors.New("not a wasm allowlist event")

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
	seen := make(map[string]struct{}, len(pubkeys))
	result := make([]string, 0, len(pubkeys))

	for _, pubkey := range pubkeys {
		trimmed := strings.TrimSpace(pubkey)
		if trimmed != "" {
			// Normalize to lowercase for consistent storage
			normalized := strings.ToLower(trimmed)
			// Deduplicate
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = struct{}{}
				result = append(result, normalized)
			}
		}
	}

	return result
}

// parseAllowlistAttributes extracts common attributes for allowlist events
func parseAllowlistAttributes(event abcitypes.Event) *AllowlistEvent {
	ae := &AllowlistEvent{EventType: EventType(event.Type)}
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "_contract_address":
			ae.Address = attr.Value
		case "action":
			ae.Action = attr.Value
		case "fp_pubkeys":
			ae.FpPubkeys = ParseAllowlistFromString(attr.Value)
		case "allow-list":
			ae.AllowList = ParseAllowlistFromString(attr.Value)
		case "num_added":
			ae.NumAdded = attr.Value
		case "num_removed":
			ae.NumRemoved = attr.Value
		case "msg_index":
			ae.MsgIndex = attr.Value
		}
	}
	return ae
}

// ParseInstantiateAllowlistEvent parses a wasm instantiate allowlist event
func ParseInstantiateAllowlistEvent(event abcitypes.Event) (*AllowlistEvent, error) {
	if EventType(event.Type) != EventWasm {
		return nil, ErrNotAllowlistEvent
	}
	ae := parseAllowlistAttributes(event)
	if ae.Action != ActionInstantiate {
		return nil, ErrNotAllowlistEvent
	}
	if len(ae.AllowList) == 0 {
		return nil, fmt.Errorf("instantiate event missing allow-list")
	}
	if ae.Address == "" {
		return nil, fmt.Errorf("missing address in allowlist event")
	}
	return ae, nil
}

// ParseAddToAllowlistEvent parses a wasm add_to_allowlist event
func ParseAddToAllowlistEvent(event abcitypes.Event) (*AllowlistEvent, error) {
	if EventType(event.Type) != EventWasmAddToAllowlist {
		return nil, ErrNotAllowlistEvent
	}
	ae := parseAllowlistAttributes(event)
	if len(ae.FpPubkeys) == 0 {
		return nil, fmt.Errorf("missing fp_pubkeys in %s event", ActionAddToAllowlist)
	}
	if ae.Address == "" {
		return nil, fmt.Errorf("missing address in allowlist event")
	}
	return ae, nil
}

// ParseRemoveFromAllowlistEvent parses a wasm remove_from_allowlist event
func ParseRemoveFromAllowlistEvent(event abcitypes.Event) (*AllowlistEvent, error) {
	if EventType(event.Type) != EventWasmRemoveFromAllowlist {
		return nil, ErrNotAllowlistEvent
	}
	ae := parseAllowlistAttributes(event)
	if len(ae.FpPubkeys) == 0 {
		return nil, fmt.Errorf("missing fp_pubkeys in %s event", ActionRemoveFromAllowlist)
	}
	if ae.Address == "" {
		return nil, fmt.Errorf("missing address in allowlist event")
	}
	return ae, nil
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
