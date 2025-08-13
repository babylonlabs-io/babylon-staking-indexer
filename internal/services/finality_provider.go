package services

import (
	"context"
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

// UpdateBabylonFinalityProviderBsnId updates all Babylon FPs that have
// empty BSN IDs with the babylon BSN ID derived from the network chain ID.
// Returns the number of finality providers updated.
// This method should only be ran once and will be removed in the future versions.
func (s *Service) UpdateBabylonFinalityProviderBsnId(ctx context.Context) (int64, error) {
	log := log.Ctx(ctx)

	// Get all finality providers
	finalityProviders, err := s.db.GetAllFinalityProviders(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get all finality providers: %w", err)
	}

	// Get network info to derive babylon BSN ID
	networkInfo, err := s.db.GetNetworkInfo(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get network info: %w", err)
	}

	// Derive babylon BSN ID from chain ID
	// For babylon chains, the BSN ID is typically the chain ID itself
	babylonBsnId := networkInfo.ChainID

	log.Info().
		Str("babylonBsnId", babylonBsnId).
		Int("totalFinalityProviders", len(finalityProviders)).
		Msg("Starting to update finality providers with missing BSN IDs")

	updatedCount := int64(0)

	// Loop through all finality providers and update those with empty BSN IDs
	for _, fp := range finalityProviders {
		if fp.BsnID == "" {
			log.Debug().
				Str("btcPk", fp.BtcPk).
				Str("newBsnId", babylonBsnId).
				Msg("Updating finality provider with missing BSN ID")

			err := s.db.UpdateFinalityProviderBsnId(ctx, fp.BtcPk, babylonBsnId)
			if err != nil {
				log.Error().
					Err(err).
					Str("btcPk", fp.BtcPk).
					Msg("Failed to update finality provider BSN ID")
				return updatedCount, fmt.Errorf("failed to update finality provider %s BSN ID: %w", fp.BtcPk, err)
			}

			updatedCount++
		}
	}

	log.Info().
		Int64("updatedCount", updatedCount).
		Msg("Successfully updated finality providers with missing BSN IDs")

	return updatedCount, nil
}
