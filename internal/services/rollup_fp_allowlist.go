package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

// processAllowlistInstantiateEvent processes contract instantiation events
func (s *Service) processAllowlistInstantiateEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	log := log.Ctx(ctx)

	// Parse the allowlist event
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		log.Error().Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse allowlist instantiate event")
		return err
	}

	// Log the parsed instantiate event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Str("action", allowlistEvent.Action).
		Interface("allowlist", allowlistEvent.AllowList).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg("Bootstrap: Allowlist instantiate event processed (logged only)")

	return nil
}

// processAddToAllowlistEvent processes specialized add to allowlist events
// TODO: Implement database persistence and business logic for add to allowlist events
func (s *Service) processAddToAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	log := log.Ctx(ctx)

	// Parse the allowlist event
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		log.Error().Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse add to allowlist event")
		return err
	}

	// Log the parsed add to allowlist event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Interface("fp_pubkeys", allowlistEvent.FpPubkeys).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg("Bootstrap: Add to allowlist event processed (logged only)")

	return nil
}

// processRemoveFromAllowlistEvent processes specialized remove from allowlist events
// TODO: Implement database persistence and business logic for remove from allowlist events
func (s *Service) processRemoveFromAllowlistEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
	log := log.Ctx(ctx)

	// Parse the allowlist event
	allowlistEvent, err := types.ParseAllowlistEvent(event)
	if err != nil {
		log.Error().Err(err).
			Str("event_type", event.Type).
			Int64("block_height", blockHeight).
			Msg("Failed to parse remove from allowlist event")
		return err
	}

	// Log the parsed remove from allowlist event details
	log.Debug().
		Str("event_type", string(allowlistEvent.EventType)).
		Str("address", allowlistEvent.Address).
		Interface("fp_pubkeys", allowlistEvent.FpPubkeys).
		Str("msg_index", allowlistEvent.MsgIndex).
		Int64("block_height", blockHeight).
		Msg("Processing allowlist event")

	// Check if we have a BSN registered for this contract address
	bsn, err := s.db.GetBSNByAddress(ctx, allowlistEvent.Address)
	if err != nil {
		log.Warn().Err(err).
			Str("address", allowlistEvent.Address).
			Msg("BSN not found for allowlist event, skipping")
		return nil // Don't error out, just skip processing
	}

	// Determine the event type for the BSN update
	var eventType string
	var pubkeys []string

	if allowlistEvent.IsInstantiateEvent() {
		eventType = "instantiate"
		pubkeys = allowlistEvent.GetPubkeys()
	} else if allowlistEvent.IsAddEvent() {
		eventType = "add_to_allowlist"
		pubkeys = allowlistEvent.FpPubkeys
	} else if allowlistEvent.IsRemoveEvent() {
		eventType = "remove_from_allowlist"
		pubkeys = allowlistEvent.FpPubkeys
	} else {
		log.Warn().
			Str("event_type", string(allowlistEvent.EventType)).
			Str("action", allowlistEvent.Action).
			Msg("Unknown allowlist event type, skipping")
		return nil
	}

	if len(pubkeys) == 0 {
		log.Debug().
			Str("event_type", string(allowlistEvent.EventType)).
			Str("action", allowlistEvent.Action).
			Msg("No pubkeys in allowlist event, skipping")
		return nil
	}

	// Compute incremental FP allowlist updates using normalized keys
	oldSet := make(map[string]struct{})
	if bsn.RollupMetadata != nil {
		for _, k := range bsn.RollupMetadata.Allowlist {
			oldSet[strings.ToLower(k)] = struct{}{}
		}
	}

	// Normalize incoming pubkeys (ParseAllowlistEvent already lowercases, this is defensive)
	normPub := make([]string, 0, len(pubkeys))
	seen := make(map[string]struct{}, len(pubkeys))
	for _, k := range pubkeys {
		l := strings.ToLower(k)
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		normPub = append(normPub, l)
	}

	added := make([]string, 0)
	removed := make([]string, 0)

	switch eventType {
	case "instantiate":
		// Full snapshot: diff new vs old
		newSet := make(map[string]struct{}, len(normPub))
		for _, k := range normPub {
			newSet[k] = struct{}{}
		}
		for k := range newSet {
			if _, ok := oldSet[k]; !ok {
				added = append(added, k)
			}
		}
		for k := range oldSet {
			if _, ok := newSet[k]; !ok {
				removed = append(removed, k)
			}
		}
	case "add_to_allowlist":
		added = normPub
	case "remove_from_allowlist":
		removed = normPub
	}

	// Persist BSN allowlist first
	err = s.db.UpdateBSNAllowlist(ctx, allowlistEvent.Address, pubkeys, eventType)
	if err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Str("event_type", eventType).
			Interface("pubkeys", pubkeys).
			Int64("block_height", blockHeight).
			Msg("Failed to update BSN allowlist")
		return fmt.Errorf("failed to update BSN allowlist: %w", err)
	}

	// Apply incremental updates to FPs
	if len(added) > 0 {
		if err := s.db.SetFPAllowlisted(ctx, bsn.ID, added, true); err != nil {
			log.Error().Err(err).
				Str("bsn_id", bsn.ID).
				Interface("added", added).
				Msg("Failed to set is_allowlisted=true for added pubkeys")
			return err
		}
	}
	if len(removed) > 0 {
		if err := s.db.SetFPAllowlisted(ctx, bsn.ID, removed, false); err != nil {
			log.Error().Err(err).
				Str("bsn_id", bsn.ID).
				Interface("removed", removed).
				Msg("Failed to set is_allowlisted=false for removed pubkeys")
			return err
		}
	}

	log.Info().
		Str("bsn_id", bsn.ID).
		Str("bsn_name", bsn.Name).
		Str("address", allowlistEvent.Address).
		Str("event_type", eventType).
		Interface("pubkeys", pubkeys).
		Int64("added", int64(len(added))).
		Int64("removed", int64(len(removed))).
		Int64("block_height", blockHeight).
		Msg("Successfully processed allowlist event and updated FP flags")

	return nil
}
