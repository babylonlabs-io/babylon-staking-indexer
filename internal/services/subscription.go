package services

import (
	"context"

	"github.com/cometbft/cometbft/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) SubscribeToBbnEvents(ctx context.Context) {
	subscriberName := "babylon-staking-indexer"
	query := "tm.event='NewBlock'"

	if !s.bbn.IsRunning() {
		log.Fatal().Msg("BBN client is not running")
	}

	eventChan, err := s.bbn.Subscribe(subscriberName, query)
	if err != nil {
		log.Fatal().Msgf("Failed to subscribe to events: %v", err)
	}

	go func() {
		for {
			select {
			case event := <-eventChan:
				newBlockEvent, ok := event.Data.(types.EventDataNewBlock)
				if !ok {
					log.Fatal().Msg("Event is not a NewBlock event")
				}

				latestHeight := newBlockEvent.Block.Height
				if latestHeight == 0 {
					log.Fatal().Msg("Event doesn't contain block height information")
				}

				// Send the latest height to the bootstrap process
				s.latestHeightChan <- latestHeight

			case <-ctx.Done():
				err := s.bbn.UnsubscribeAll(subscriberName)
				if err != nil {
					log.Error().Msgf("Failed to unsubscribe from events: %v", err)
				}
				return
			}
		}
	}()
}
