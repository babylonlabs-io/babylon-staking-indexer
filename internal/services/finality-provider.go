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
	if err := validateFinalityProviderCreatedEvent(newFinalityProvider); err != nil {
		return err
	}
	if err := s.db.SaveNewFinalityProvider(
		ctx, model.FromEventFinalityProviderCreated(newFinalityProvider),
	); err != nil {
		if db.IsDuplicateKeyError(err) {
			// Finality provider already exists, ignore the event
			return nil
		}
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save new finality provider: %w", err),
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
	if err := validateFinalityProviderEditedEvent(finalityProviderEdited); err != nil {
		return err
	}
	if err := s.db.UpdateFinalityProviderDetailsFromEvent(
		ctx, model.FromEventFinalityProviderEdited(finalityProviderEdited),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update finality provider details: %w", err),
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
	if err := validateFinalityProviderStateChangeEvent(finalityProviderStateChange); err != nil {
		return err
	}

	// Check FP exists
	_, dbErr := s.db.GetFinalityProviderByBtcPk(ctx, finalityProviderStateChange.BtcPk)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get finality provider by btc public key: %w", dbErr),
		)
	}

	// If all validations pass, update the finality provider state
	if err := s.db.UpdateFinalityProviderState(
		ctx, finalityProviderStateChange.BtcPk, finalityProviderStateChange.NewState,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update finality provider state: %w", err),
		)
	}
	return nil
}

// validateFinalityProviderCreatedEvent validates properties of
// the new finality provider event and returns an error if the event is invalid.
func validateFinalityProviderCreatedEvent(
	fpCreated *bbntypes.EventFinalityProviderCreated,
) *types.Error {
	// TODO: Implement validation logic
	return nil
}

// validateFinalityProviderEditedEvent validates properties of
// the finality provider edited event and returns an error if the event is invalid.
func validateFinalityProviderEditedEvent(
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

func validateFinalityProviderStateChangeEvent(
	fpStateChange *bbntypes.EventFinalityProviderStatusChange,
) *types.Error {
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
