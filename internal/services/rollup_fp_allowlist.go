package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

// processAllowlistInstantiateEvent processes contract instantiation events
func (s *Service) processAllowlistInstantiateEvent(ctx context.Context, event abcitypes.Event, blockHeight int64) error {
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

	// Compute changes
	added, removed := s.computeAllowlistChanges(bsn, pubkeys, eventType)

	// Persist BSN allowlist
	if err := s.db.UpdateBSNAllowlist(ctx, allowlistEvent.Address, pubkeys, eventType); err != nil {
		log.Error().Err(err).
			Str("address", allowlistEvent.Address).
			Str("event_type", eventType).
			Msg("Failed to update BSN allowlist")
		return fmt.Errorf("failed to update BSN allowlist: %w", err)
	}

	// Update FP allowlist flags
	if err := s.updateFPAllowlistFlags(ctx, bsn.ID, added, removed); err != nil {
		return err
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

// updateFPAllowlistFlags updates the allowlist flags for finality providers
func (s *Service) updateFPAllowlistFlags(ctx context.Context, bsnID string, added, removed []string) error {
	log := log.Ctx(ctx)

	if len(added) > 0 {
		if err := s.db.SetFPAllowlisted(ctx, bsnID, added, true); err != nil {
			log.Error().Err(err).
				Str("bsn_id", bsnID).
				Interface("added", added).
				Msg("Failed to set is_allowlisted=true for added pubkeys")
			return err
		}
	}

	if len(removed) > 0 {
		if err := s.db.SetFPAllowlisted(ctx, bsnID, removed, false); err != nil {
			log.Error().Err(err).
				Str("bsn_id", bsnID).
				Interface("removed", removed).
				Msg("Failed to set is_allowlisted=false for removed pubkeys")
			return err
		}
	}

	return nil
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
