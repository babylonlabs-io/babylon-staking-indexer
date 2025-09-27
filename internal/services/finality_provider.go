package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) processNewFinalityProviderEvent(
	ctx context.Context, event abcitypes.Event,
) error {
	newFinalityProvider, err := parseEvent[*bbntypes.EventFinalityProviderCreated](
		types.EventFinalityProviderCreatedType, event,
	)
	if err != nil {
		return err
	}

	log := log.Ctx(ctx)
	log.Info().Interface("event", newFinalityProvider).Msg("FinalityProvider created")

	if validationErr := s.validateFinalityProviderCreatedEvent(newFinalityProvider); validationErr != nil {
		return validationErr
	}

	if dbErr := s.db.SaveNewFinalityProvider(
		ctx, model.FromEventFinalityProviderCreated(newFinalityProvider),
	); dbErr != nil {
		if db.IsDuplicateKeyError(dbErr) {
			// Finality provider already exists, ignore the event
			log.Debug().
				Str("btcPk", newFinalityProvider.BtcPkHex).
				Msg("Ignoring EventFinalityProviderCreated because finality provider already exists")
			return nil
		}
		return fmt.Errorf("failed to save new finality provider: %w", dbErr)
	}

	return nil
}

func (s *Service) processFinalityProviderEditedEvent(
	ctx context.Context, event abcitypes.Event,
) error {
	finalityProviderEdited, err := parseEvent[*bbntypes.EventFinalityProviderEdited](
		types.EventFinalityProviderEditedType, event,
	)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Interface("event", finalityProviderEdited).Msg("FinalityProvider edited")

	if validationErr := s.validateFinalityProviderEditedEvent(finalityProviderEdited); validationErr != nil {
		return validationErr
	}

	if dbErr := s.db.UpdateFinalityProviderDetailsFromEvent(
		ctx, model.FromEventFinalityProviderEdited(finalityProviderEdited),
	); dbErr != nil {
		return fmt.Errorf("failed to update finality provider details: %w", dbErr)
	}

	return nil
}

func (s *Service) processFinalityProviderStateChangeEvent(
	ctx context.Context, event abcitypes.Event,
) error {
	finalityProviderStateChange, err := parseEvent[*bbntypes.EventFinalityProviderStatusChange](
		types.EventFinalityProviderStatusChange, event,
	)
	if err != nil {
		return err
	}

	log.Ctx(ctx).Info().Interface("event", finalityProviderStateChange).Msg("FinalityProvider status changed")

	if validationErr := s.validateFinalityProviderStateChangeEvent(ctx, finalityProviderStateChange); validationErr != nil {
		if errors.Is(validationErr, types.ErrFinalityProviderAlreadySlashed) {
			log.Ctx(ctx).Warn().
				Str("btcPk", finalityProviderStateChange.BtcPk).
				Str("newState", finalityProviderStateChange.NewState).
				Err(validationErr).
				Msg("Finality provider is already slashed, cannot change state, ignoring event")
			return nil
		}
		return validationErr
	}

	// If all validations pass, update the finality provider state
	if dbErr := s.db.UpdateFinalityProviderState(
		ctx, finalityProviderStateChange.BtcPk, finalityProviderStateChange.NewState,
	); dbErr != nil {
		return fmt.Errorf("failed to update finality provider state: %w", dbErr)
	}
	return nil
}

// validateFinalityProviderCreatedEvent validates properties of
// the new finality provider event and returns an error if the event is invalid.
func (s *Service) validateFinalityProviderCreatedEvent(
	fpCreated *bbntypes.EventFinalityProviderCreated,
) error {
	if fpCreated.BtcPkHex == "" {
		return fmt.Errorf("finality provider created event missing btc public key")
	}
	return nil
}

// validateFinalityProviderEditedEvent validates properties of
// the finality provider edited event and returns an error if the event is invalid.
func (s *Service) validateFinalityProviderEditedEvent(
	fpEdited *bbntypes.EventFinalityProviderEdited,
) error {
	if fpEdited.BtcPkHex == "" {
		return fmt.Errorf("finality provider edited event missing btc public key")
	}
	// TODO: Implement validation logic
	return nil
}

func (s *Service) validateFinalityProviderStateChangeEvent(
	ctx context.Context,
	fpStateChange *bbntypes.EventFinalityProviderStatusChange,
) error {
	// Check FP exists
	fp, dbErr := s.db.GetFinalityProviderByBtcPk(ctx, fpStateChange.BtcPk)
	if dbErr != nil {
		return fmt.Errorf("failed to get finality provider by btc public key: %w", dbErr)
	}

	if fpStateChange.BtcPk == "" {
		return fmt.Errorf("finality provider State change event missing btc public key")
	}
	if fpStateChange.NewState == "" {
		return fmt.Errorf("finality provider State change event missing State")
	}

	// Check if the finality provider is already slashed. No point in changing
	// the state of a slashed finality provider.
	if fp.State == bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_SLASHED.String() {
		return types.ErrFinalityProviderAlreadySlashed
	}

	return nil
}
