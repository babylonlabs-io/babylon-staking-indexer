package services

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

// processAllowlistInstantiateEvent processes contract instantiation events
// TODO: Implement database persistence and business logic for instantiate events
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
		Msg("Bootstrap: Remove from allowlist event processed (logged only)")

	return nil
}
