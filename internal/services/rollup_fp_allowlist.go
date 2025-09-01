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
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		if errors.Is(err, types.ErrNotAllowlistEvent) {
			// Not an allowlist event, skip silently
			log.Ctx(ctx).Debug().
				Str("event_type", event.Type).
				Int64("block_height", blockHeight).
				Msg("Skipping non-allowlist wasm event")
			return nil
		}
		// Other parsing errors should be returned as errors
		log.Ctx(ctx).Error().
			Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse wasm event as allowlist event")
		return fmt.Errorf("failed to parse wasm allowlist event: %w", err)
	}

	// It's an allowlist instantiate event, process it
	return s.processInstantiateAllowlistEvent(ctx, allowlistEvent, blockHeight)
}

// processAddToAllowlistEvent processes specialized add to allowlist events
func (s *Service) processAddToAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		log.Ctx(ctx).Error().
			Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse add to allowlist event")
		return fmt.Errorf("failed to parse add to allowlist event: %w", err)
	}

	return s.processAddAllowlistEvent(ctx, allowlistEvent, blockHeight)
}

// processRemoveFromAllowlistEvent processes specialized remove from allowlist events
func (s *Service) processRemoveFromAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		log.Ctx(ctx).Error().
			Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse remove from allowlist event")
		return fmt.Errorf("failed to parse remove from allowlist event: %w", err)
	}

	return s.processRemoveAllowlistEvent(ctx, allowlistEvent, blockHeight)
}

// processInstantiateAllowlistEvent handles contract instantiation with allowlist
func (s *Service) processInstantiateAllowlistEvent(ctx context.Context, allowlistEvent *types.AllowlistEvent, blockHeight int64) error {
	log := log.Ctx(ctx)

	// Validate we have the required data for instantiate
	if len(allowlistEvent.AllowList) == 0 {
		log.Debug().
			Str("address", allowlistEvent.Address).
			Int64("block_height", blockHeight).
			Msg("Instantiate event has empty allowlist, skipping")
		return nil
	}

	// Log event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Str("action", allowlistEvent.Action).
		Interface("allowlist", allowlistEvent.AllowList).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg("Processing allowlist instantiate event")

	// Check if we have a BSN registered for this contract address
	bsn, err := s.db.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("BSN not found for instantiate event")
		return fmt.Errorf("BSN not found for instantiate event with address %s: %w", allowlistEvent.Address, err)
	}

	// For instantiate, we replace the entire allowlist
	newAllowlist := normalizePubkeys(allowlistEvent.AllowList)

	// Compute changes for logging
	added, removed := s.computeAllowlistChanges(bsn, allowlistEvent.AllowList, types.ActionInstantiate)

	// Persist BSN allowlist
	if err := s.db.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("Failed to update BSN allowlist for instantiate")
		return fmt.Errorf("failed to update BSN allowlist for instantiate: %w", err)
	}

	log.Info().
		Str("bsn_id", bsn.ID).
		Str("bsn_name", bsn.Name).
		Str("address", allowlistEvent.Address).
		Interface("allowlist", newAllowlist).
		Int("added", len(added)).
		Int("removed", len(removed)).
		Int64("block_height", blockHeight).
		Msg("Successfully instantiated BSN with allowlist")

	return nil
}

// processAddAllowlistEvent handles adding finality providers to the allowlist
func (s *Service) processAddAllowlistEvent(ctx context.Context, allowlistEvent *types.AllowlistEvent, blockHeight int64) error {
	log := log.Ctx(ctx)

	// Validate we have pubkeys to add
	if len(allowlistEvent.FpPubkeys) == 0 {
		log.Debug().
			Str("address", allowlistEvent.Address).
			Int64("block_height", blockHeight).
			Msg("Add to allowlist event has no pubkeys, skipping")
		return nil
	}

	// Log event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Interface("fp_pubkeys", allowlistEvent.FpPubkeys).
		Str("num_added", allowlistEvent.NumAdded).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg("Processing add to allowlist event")

	// Check if we have a BSN registered for this contract address
	bsn, err := s.db.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("BSN not found for add to allowlist event")
		return fmt.Errorf("BSN not found for add to allowlist event with address %s: %w", allowlistEvent.Address, err)
	}

	// Compute the new allowlist by adding the pubkeys
	newAllowlist := s.computeNewAllowlist(bsn, allowlistEvent.FpPubkeys, types.ActionAddToAllowlist)

	// Persist BSN allowlist
	if err := s.db.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("Failed to update BSN allowlist for add")
		return fmt.Errorf("failed to update BSN allowlist for add: %w", err)
	}

	log.Info().
		Str("bsn_id", bsn.ID).
		Str("bsn_name", bsn.Name).
		Str("address", allowlistEvent.Address).
		Interface("added_pubkeys", allowlistEvent.FpPubkeys).
		Int("num_added", len(allowlistEvent.FpPubkeys)).
		Int64("block_height", blockHeight).
		Msg("Successfully added to BSN allowlist")

	return nil
}

// processRemoveAllowlistEvent handles removing finality providers from the allowlist
func (s *Service) processRemoveAllowlistEvent(ctx context.Context, allowlistEvent *types.AllowlistEvent, blockHeight int64) error {
	log := log.Ctx(ctx)

	// Validate we have pubkeys to remove
	if len(allowlistEvent.FpPubkeys) == 0 {
		log.Debug().
			Str("address", allowlistEvent.Address).
			Int64("block_height", blockHeight).
			Msg("Remove from allowlist event has no pubkeys, skipping")
		return nil
	}

	// Log event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Interface("fp_pubkeys", allowlistEvent.FpPubkeys).
		Str("num_removed", allowlistEvent.NumRemoved).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg("Processing remove from allowlist event")

	// Check if we have a BSN registered for this contract address
	bsn, err := s.db.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("BSN not found for remove from allowlist event")
		return fmt.Errorf("BSN not found for remove from allowlist event with address %s: %w", allowlistEvent.Address, err)
	}

	// Compute the new allowlist by removing the pubkeys
	newAllowlist := s.computeNewAllowlist(bsn, allowlistEvent.FpPubkeys, types.ActionRemoveFromAllowlist)

	// Persist BSN allowlist
	if err := s.db.UpdateBSNAllowlist(ctx, allowlistEvent.Address, newAllowlist); err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("Failed to update BSN allowlist for remove")
		return fmt.Errorf("failed to update BSN allowlist for remove: %w", err)
	}

	log.Info().
		Str("bsn_id", bsn.ID).
		Str("bsn_name", bsn.Name).
		Str("address", allowlistEvent.Address).
		Interface("removed_pubkeys", allowlistEvent.FpPubkeys).
		Int("num_removed", len(allowlistEvent.FpPubkeys)).
		Int64("block_height", blockHeight).
		Msg("Successfully removed from BSN allowlist")

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
