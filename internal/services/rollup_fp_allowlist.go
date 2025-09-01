package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

// processWasmEvent processes wasm events and filters for allowlist-related instantiate events
func (s *Service) processWasmEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	// Try to parse as an allowlist event
	_, err := types.ParseAllowlistEvent(event)
	if err != nil {
		if errors.Is(err, types.ErrNotAllowlistEvent) {
			// Not an allowlist event, skip silently
			log.Ctx(ctx).Debug().
				Str("event_type", event.Type).
				Int64("block_height", blockHeight).
				Msg("Skipping non-allowlist wasm event")
			return nil
		}
		// Other parsing errors should be logged
		log.Ctx(ctx).Warn().
			Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse wasm event as allowlist event")
		return nil
	}

	// It's an allowlist instantiate event, process it
	return s.processAllowlistEvent(ctx, event, blockHeight, types.ActionInstantiate, "Processing allowlist instantiate event")
}

// processAddToAllowlistEvent processes specialized add to allowlist events
func (s *Service) processAddToAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	return s.processAllowlistEvent(ctx, event, blockHeight, types.ActionAddToAllowlist, "Processing add to allowlist event")
}

// processRemoveFromAllowlistEvent processes specialized remove from allowlist events
func (s *Service) processRemoveFromAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	return s.processAllowlistEvent(ctx, event, blockHeight, types.ActionRemoveFromAllowlist, "Processing remove from allowlist event")
}

// processAllowlistEvent handles the common allowlist processing logic
func (s *Service) processAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64, eventType string, logMsg string) error {
	log := log.Ctx(ctx)

	// Parse the allowlist event
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		log.Error().Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msgf("Failed to parse allowlist event: %s", eventType)
		return err
	}

	// Log event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Str("action", allowlistEvent.Action).
		Interface("fp_pubkeys", allowlistEvent.FpPubkeys).
		Interface("allowlist", allowlistEvent.AllowList).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg(logMsg)

	// Check if we have a BSN registered for this contract address
	bsn, err := s.db.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		log.Warn().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("BSN not found for allowlist event, skipping")
		return nil
	}

	// Get pubkeys based on event type
	var pubkeys []string
	switch eventType {
	case types.ActionInstantiate:
		pubkeys = allowlistEvent.GetPubkeys()
	case types.ActionAddToAllowlist, types.ActionRemoveFromAllowlist:
		pubkeys = allowlistEvent.FpPubkeys
	default:
		log.Warn().
			Str("event_type", string(allowlistEvent.EventType)).
			Str("action", allowlistEvent.Action).
			Msgf("Unknown allowlist event type: %s", eventType)
		return nil
	}

	if len(pubkeys) == 0 {
		log.Debug().
			Str("event_type", string(allowlistEvent.EventType)).
			Str("action", allowlistEvent.Action).
			Msg("No pubkeys in allowlist event, skipping")
		return nil
	}

	// Compute the new allowlist based on event type
	newAllowlist := s.computeNewAllowlist(bsn, pubkeys, eventType)

	// Compute changes
	added, removed := s.computeAllowlistChanges(bsn, pubkeys, eventType)

	// Persist BSN allowlist
	if err := s.db.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Str("event_type", eventType).
			Msg("Failed to update BSN allowlist")
		return fmt.Errorf("failed to update BSN allowlist: %w", err)
	}

	log.Info().
		Str("bsn_id", bsn.ID).
		Str("bsn_name", bsn.Name).
		Str("address", allowlistEvent.Address).
		Str("event_type", eventType).
		Interface("pubkeys", pubkeys).
		Int("added", len(added)).
		Int("removed", len(removed)).
		Int64("block_height", blockHeight).
		Msg("Successfully processed allowlist event")

	return nil
}

// computeAllowlistChanges calculates which pubkeys should be added or removed
func (s *Service) computeAllowlistChanges(bsn *model.BSN, pubkeys []string, eventType string) ([]string, []string) {
	// Build old allowlist set
	oldSet := make(map[string]struct{})
	if bsn.RollupMetadata != nil {
		for _, k := range bsn.RollupMetadata.Allowlist {
			oldSet[strings.ToLower(k)] = struct{}{}
		}
	}

	// Normalize new pubkeys
	normPub := normalizePubkeys(pubkeys)

	switch eventType {
	case types.ActionInstantiate:
		// Full snapshot: diff new vs old
		newSet := make(map[string]struct{}, len(normPub))
		for _, k := range normPub {
			newSet[k] = struct{}{}
		}

		added := make([]string, 0)
		for k := range newSet {
			if _, ok := oldSet[k]; !ok {
				added = append(added, k)
			}
		}

		removed := make([]string, 0)
		for k := range oldSet {
			if _, ok := newSet[k]; !ok {
				removed = append(removed, k)
			}
		}
		return added, removed

	case types.ActionAddToAllowlist:
		return normPub, nil

	case types.ActionRemoveFromAllowlist:
		return nil, normPub

	default:
		return nil, nil
	}
}

// normalizePubkeys normalizes and deduplicates pubkeys
func normalizePubkeys(pubkeys []string) []string {
	seen := make(map[string]struct{}, len(pubkeys))
	result := make([]string, 0, len(pubkeys))

	for _, pk := range pubkeys {
		l := strings.ToLower(pk)
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		result = append(result, l)
	}

	return result
}

// computeNewAllowlist computes the final allowlist based on the event type
func (s *Service) computeNewAllowlist(bsn *model.BSN, pubkeys []string, eventType string) []string {
	// Get current allowlist
	currentAllowlist := make([]string, 0)
	if bsn.RollupMetadata != nil && bsn.RollupMetadata.Allowlist != nil {
		currentAllowlist = bsn.RollupMetadata.Allowlist
	}

	// Normalize new pubkeys
	normPubkeys := normalizePubkeys(pubkeys)

	switch eventType {
	case types.ActionInstantiate:
		// For instantiate, replace entire allowlist
		return normPubkeys

	case types.ActionAddToAllowlist:
		// Merge with existing allowlist
		allowlistMap := make(map[string]struct{})
		for _, pk := range currentAllowlist {
			allowlistMap[strings.ToLower(pk)] = struct{}{}
		}
		for _, pk := range normPubkeys {
			allowlistMap[pk] = struct{}{}
		}

		// Convert back to slice
		newAllowlist := make([]string, 0, len(allowlistMap))
		for pk := range allowlistMap {
			newAllowlist = append(newAllowlist, pk)
		}
		return newAllowlist

	case types.ActionRemoveFromAllowlist:
		// Remove from existing allowlist
		toRemove := make(map[string]struct{})
		for _, pk := range normPubkeys {
			toRemove[pk] = struct{}{}
		}

		newAllowlist := make([]string, 0)
		for _, pk := range currentAllowlist {
			normalized := strings.ToLower(pk)
			if _, shouldRemove := toRemove[normalized]; !shouldRemove {
				newAllowlist = append(newAllowlist, normalized)
			}
		}
		return newAllowlist

	default:
		// Unknown event type, return current allowlist unchanged
		return currentAllowlist
	}
}
