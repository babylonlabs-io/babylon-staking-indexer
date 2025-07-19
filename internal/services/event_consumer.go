package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	_ "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	btcstkconsumer "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
)

func (s *Service) processEventConsumerRegisteredEvent(ctx context.Context, rawEvent abcitypes.Event) error {
	event, err := parseEvent[*btcstkconsumer.EventConsumerRegistered](
		types.EventConsumerRegistered, rawEvent,
	)
	if err != nil {
		return err
	}

	if dbErr := s.db.SaveBSN(
		ctx, model.FromEventConsumerRegistered(event),
	); dbErr != nil {
		return fmt.Errorf("failed to save new event consumer: %w", dbErr)
	}

	return nil
}
