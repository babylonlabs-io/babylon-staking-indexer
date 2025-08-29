package model

import (
	"strings"

	btcstkconsumer "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

type BSN struct {
	ID             string         `bson:"_id"`
	Name           string         `bson:"name"`
	Description    string         `bson:"description"`
	Type           string         `bson:"type"`
	RollupMetadata *ETHL2Metadata `bson:"rollup_metadata"`
}

type ETHL2Metadata struct {
	FinalityContractAddress string   `bson:"finality_contract_address"`
	Allowlist               []string `bson:"allowlist,omitempty"` // array of FP BTC pubkeys hex
}

func FromEventConsumerRegistered(event *btcstkconsumer.EventConsumerRegistered) *BSN {
	var rollupMetadata *ETHL2Metadata
	if event.RollupConsumerMetadata != nil {
		rollupMetadata = &ETHL2Metadata{
			FinalityContractAddress: event.RollupConsumerMetadata.FinalityContractAddress,
		}
	}

	return &BSN{
		ID:             event.ConsumerId,
		Name:           event.ConsumerName,
		Description:    event.ConsumerDescription,
		Type:           event.ConsumerType.String(),
		RollupMetadata: rollupMetadata,
	}
}

// UpdateAllowlistFromEvent updates the allowlist based on an allowlist event
func (b *BSN) UpdateAllowlistFromEvent(pubkeys []string, eventType string) {
	if b.RollupMetadata == nil {
		b.RollupMetadata = &ETHL2Metadata{}
	}

	norm := normalizePubkeys(pubkeys)

	switch eventType {
	case "instantiate":
		if len(norm) > 0 {
			b.RollupMetadata.Allowlist = norm
		}
	case "add_to_allowlist":
		if len(norm) > 0 {
			b.mergeAllowlist(norm)
		}
	case "remove_from_allowlist":
		b.removeFromAllowlist(norm)
	}
}

func normalizePubkeys(pubkeys []string) []string {
	seen := make(map[string]struct{}, len(pubkeys))
	out := make([]string, 0, len(pubkeys))
	for _, pk := range pubkeys {
		l := strings.ToLower(pk)
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	return out
}

// mergeAllowlist adds new pubkeys to the allowlist while avoiding duplicates
func (b *BSN) mergeAllowlist(newPubkeys []string) {
	if b.RollupMetadata == nil {
		return
	}

	b.RollupMetadata.Allowlist = normalizePubkeys(b.RollupMetadata.Allowlist)
	present := make(map[string]struct{}, len(b.RollupMetadata.Allowlist))
	for _, pk := range b.RollupMetadata.Allowlist {
		present[pk] = struct{}{}
	}

	for _, pk := range newPubkeys {
		l := strings.ToLower(pk)
		if _, ok := present[l]; !ok {
			b.RollupMetadata.Allowlist = append(b.RollupMetadata.Allowlist, l)
			present[l] = struct{}{}
		}
	}
}

// removeFromAllowlist removes specified pubkeys from the allowlist
func (b *BSN) removeFromAllowlist(pubkeysToRemove []string) {
	if b.RollupMetadata == nil {
		return
	}

	b.RollupMetadata.Allowlist = normalizePubkeys(b.RollupMetadata.Allowlist)
	toRemove := make(map[string]struct{}, len(pubkeysToRemove))
	for _, pk := range pubkeysToRemove {
		toRemove[strings.ToLower(pk)] = struct{}{}
	}

	var filtered []string
	for _, pk := range b.RollupMetadata.Allowlist {
		if _, rm := toRemove[pk]; !rm {
			filtered = append(filtered, pk)
		}
	}

	b.RollupMetadata.Allowlist = filtered
}

// HasAllowlistForContract returns true if this BSN has an allowlist for the given contract address
func (b *BSN) HasAllowlistForContract(address string) bool {
	return b.RollupMetadata != nil &&
		b.RollupMetadata.FinalityContractAddress == address &&
		len(b.RollupMetadata.Allowlist) > 0
}
