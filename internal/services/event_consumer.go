package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	btcstkconsumer "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
	"slices"
)

func (s *Service) processEventConsumerRegisteredEvent(ctx context.Context, rawEvent abcitypes.Event) error {
	event, err := parseEvent[*btcstkconsumer.EventConsumerRegistered](
		types.EventConsumerRegistered, rawEvent,
	)
	if err != nil {
		return err
	}

	log := log.Ctx(ctx)

	err = s.validateEventConsumerRegisteredEvent(event)
	if err != nil {
		return err
	}

	if dbErr := s.db.SaveNewEventConsumer(
		ctx, model.FromEventConsumerRegistered(event),
	); dbErr != nil {
		if db.IsDuplicateKeyError(dbErr) {
			// Finality provider already exists, ignore the event
			log.Debug().
				Msg("Ignoring EventConsumerRegistered because event consumer already exists")
			return nil
		}

		return fmt.Errorf("failed to save new event consumer: %w", dbErr)
	}

	return nil
}

func (s *Service) validateEventConsumerRegisteredEvent(event *btcstkconsumer.EventConsumerRegistered) error {
	supportedTypes := []btcstkconsumer.ConsumerType{
		btcstkconsumer.ConsumerType_COSMOS,
		btcstkconsumer.ConsumerType_ETH_L2,
	}
	if !slices.Contains(supportedTypes, event.ConsumerType) {
		return fmt.Errorf("unknown consumer type: %v", event.ConsumerType)
	}

	return nil
}
