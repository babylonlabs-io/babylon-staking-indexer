package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

const (
	EventFinalityProviderCreatedType  EventTypes = "babylon.btcstaking.v1.EventFinalityProviderCreated"
	EventFinalityProviderEditedType   EventTypes = "babylon.btcstaking.v1.EventFinalityProviderEdited"
	EventFinalityProviderStatusChange EventTypes = "babylon.btcstaking.v1.EventFinalityProviderStatusChange"
)

func (s *Service) processNewFinalityProviderEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	newFinalityProvider, err := parseEvent[*bbntypes.EventFinalityProviderCreated](
		EventFinalityProviderCreatedType, event,
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
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save new finality provider: %w", dbErr),
		)
	}

	return nil
}

func (s *Service) processFinalityProviderEditedEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	finalityProviderEdited, err := parseEvent[*bbntypes.EventFinalityProviderEdited](
		EventFinalityProviderEditedType, event,
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
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update finality provider details: %w", dbErr),
		)
	}

	return nil
}

func (s *Service) processFinalityProviderStateChangeEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	finalityProviderStateChange, err := parseEvent[*bbntypes.EventFinalityProviderStatusChange](
		EventFinalityProviderStatusChange, event,
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
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update finality provider state: %w", dbErr),
		)
	}
	return nil
}

// validateFinalityProviderCreatedEvent validates properties of
// the new finality provider event and returns an error if the event is invalid.
func (s *Service) validateFinalityProviderCreatedEvent(
	fpCreated *bbntypes.EventFinalityProviderCreated,
) *types.Error {
	if fpCreated.BtcPkHex == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"finality provider created event missing btc public key",
		)
	}
	return nil
}

// validateFinalityProviderEditedEvent validates properties of
// the finality provider edited event and returns an error if the event is invalid.
func (s *Service) validateFinalityProviderEditedEvent(
	fpEdited *bbntypes.EventFinalityProviderEdited,
) *types.Error {
	if fpEdited.BtcPkHex == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"finality provider edited event missing btc public key",
		)
	}
	// TODO: Implement validation logic
	return nil
}

func (s *Service) validateFinalityProviderStateChangeEvent(
	ctx context.Context,
	fpStateChange *bbntypes.EventFinalityProviderStatusChange,
) *types.Error {
	// Check FP exists
	_, dbErr := s.db.GetFinalityProviderByBtcPk(ctx, fpStateChange.BtcPk)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get finality provider by btc public key: %w", dbErr),
		)
	}

	if fpStateChange.BtcPk == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"finality provider State change event missing btc public key",
		)
	}
	if fpStateChange.NewState == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"finality provider State change event missing State",
		)
	}

	return nil
}
