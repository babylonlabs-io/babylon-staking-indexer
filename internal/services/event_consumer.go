package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	_ "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	btcstkconsumer "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) processEventConsumerRegisteredEvent(ctx context.Context, rawEvent abcitypes.Event) error {
	event, err := parseEvent[*btcstkconsumer.EventConsumerRegistered](
		types.EventConsumerRegistered, rawEvent,
	)
	if err != nil {
		return err
	}

	log := log.Ctx(ctx)

	if dbErr := s.db.SaveBSN(
		ctx, model.FromEventConsumerRegistered(event),
	); dbErr != nil {
		if db.IsDuplicateKeyError(dbErr) {
			// Event consumer already exists, ignore the event
			log.Debug().
				Msg("Ignoring EventConsumerRegistered because event consumer already exists")
			return nil
		}

		return fmt.Errorf("failed to save new event consumer: %w", dbErr)
	}

	return nil
}
