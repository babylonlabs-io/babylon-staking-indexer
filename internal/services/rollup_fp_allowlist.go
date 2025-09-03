package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

// processWasmEvent processes wasm events and filters for allowlist-related instantiate events
func (s *Service) processWasmEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	// Try to parse as an allowlist instantiate event specifically
	allowlistEvent, err := types.ParseInstantiateAllowlistEvent(event)
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
	newAllowlist := allowlistEvent.AllowList

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
		Int("allowlist_size", len(newAllowlist)).
		Int64("block_height", blockHeight).
		Msg("Successfully instantiated BSN allowlist")

	return nil
}

// processAddAllowlistEvent handles adding finality providers to the allowlist
func (s *Service) processAddAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	log := log.Ctx(ctx)

	allowlistEvent, err := types.ParseAddToAllowlistEvent(event)
	if err != nil {
		log.Error().
			Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse add to allowlist event")
		return fmt.Errorf("failed to parse add to allowlist event: %w", err)
	}

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

	addedCount := 0
	for _, pk := range allowlistEvent.FpPubkeys {
		if _, ok := existing[pk]; !ok {
			existing[pk] = struct{}{}
			newAllowlist = append(newAllowlist, pk)
			addedCount++
		}
	}

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
		Int("added_count", addedCount).
		Int("allowlist_size", len(newAllowlist)).
		Int64("block_height", blockHeight).
		Msg("Successfully added to BSN allowlist")

	return nil
}

// processRemoveAllowlistEvent handles removing finality providers from the allowlist
func (s *Service) processRemoveAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	log := log.Ctx(ctx)

	allowlistEvent, err := types.ParseRemoveFromAllowlistEvent(event)
	if err != nil {
		log.Error().
			Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse remove from allowlist event")
		return fmt.Errorf("failed to parse remove from allowlist event: %w", err)
	}

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
	removedCount := 0
	for _, pk := range currentAllowlist {
		if _, remove := toRemove[pk]; !remove {
			newAllowlist = append(newAllowlist, pk)
		} else {
			removedCount++
		}
	}

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
		Int("removed_count", removedCount).
		Int("allowlist_size", len(newAllowlist)).
		Int64("block_height", blockHeight).
		Msg("Successfully removed from BSN allowlist")

	return nil
}
