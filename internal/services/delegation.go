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
	EventBTCDelegationCreated                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationCreated"
	EventBTCDelegationInclusionProofReceived EventTypes = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventTypes = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationExpired"
)

func (s *Service) processNewBTCDelegationEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	log.Debug().Msg("Processing new BTC delegation event")
	newDelegation, err := parseEvent[*bbntypes.EventBTCDelegationCreated](
		EventBTCDelegationCreated, event,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse BTC delegation event")
		return err
	}
	if err := validateBTCDelegationCreatedEvent(newDelegation); err != nil {
		log.Error().Err(err).Msg("Failed to validate BTC delegation event")
		return err
	}
	log.Debug().Interface("newDelegation", newDelegation).Msg("Saving new BTC delegation")
	if err := s.db.SaveNewBTCDelegation(
		ctx, model.FromEventBTCDelegationCreated(newDelegation),
	); err != nil {
		if db.IsDuplicateKeyError(err) {
			log.Info().Msg("BTC delegation already exists, ignoring the event")
			return nil
		}
		log.Error().Err(err).Msg("Failed to save new BTC delegation")
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save new BTC delegation: %w", err),
		)
	}

	log.Info().Msg("Successfully processed and saved new BTC delegation")
	return nil
}

// You'll need to implement these functions:
func validateBTCDelegationCreatedEvent(event *bbntypes.EventBTCDelegationCreated) *types.Error {
	// Implement validation logic here
	return nil
}
