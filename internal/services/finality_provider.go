package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

	if validationErr := s.validateFinalityProviderStateChangeEvent(ctx, finalityProviderStateChange); validationErr != nil {
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
	_, dbErr := s.db.GetFinalityProviderByBtcPk(ctx, fpStateChange.BtcPk)
	if dbErr != nil {
		return fmt.Errorf("failed to get finality provider by btc public key: %w", dbErr)
	}

	if fpStateChange.BtcPk == "" {
		return fmt.Errorf("finality provider State change event missing btc public key")
	}
	if fpStateChange.NewState == "" {
		return fmt.Errorf("finality provider State change event missing State")
	}

	return nil
}
