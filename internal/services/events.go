package services

import (
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

type EventCategory string

const (
	BlockCategory EventCategory = "block"
	TxCategory    EventCategory = "tx"
)

type BbnEvent struct {
	Category EventCategory
	Event    abcitypes.Event
}

func NewBbnEvent(category EventCategory, event abcitypes.Event) BbnEvent {
	return BbnEvent{
		Category: category,
		Event:    event,
	}
}

// startBbnEventProcessor continuously listens for events from the channel and
// processes them in the main thread
func (s *Service) startBbnEventProcessor() {
	for event := range s.bbnEventProcessor {
		s.processEvent(event)
	}
}

// Entry point for processing events
func (s *Service) processEvent(event BbnEvent) {
	switch event.Category {
	case BlockCategory:
		s.processBbnBlockEvent(event.Event)
	case TxCategory:
		s.processBbnTxEvent(event.Event)
	default:
		log.Fatal().Msgf("Unknown event category: %s", event.Category)
	}
}

func (s *Service) processBbnTxEvent(event abcitypes.Event) {
	switch event.Type {
	case "place_holder_1":
		log.Info().Msgf("Processing place_holder_1 event")
	}
}

func (s *Service) processBbnBlockEvent(event abcitypes.Event) {
	switch event.Type {
	case "place_holder_2":
		log.Info().Msgf("Processing place_holder_2 event")
	}
}
