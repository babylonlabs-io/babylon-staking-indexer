package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

type EventTypes string

type EventCategory string

const (
	BlockCategory          EventCategory = "block"
	TxCategory             EventCategory = "tx"
	eventProcessingTimeout time.Duration = 30 * time.Second
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
func (s *Service) startBbnEventProcessor(ctx context.Context) {
	for event := range s.bbnEventProcessor {
		if event.Event.Type == "" {
			log.Warn().Msg("Empty event received, skipping")
			continue
		}
		// Create a new context with a timeout for each event
		ctx, cancel := context.WithTimeout(context.Background(), eventProcessingTimeout)
		defer cancel()
		s.processEvent(ctx, event)
	}
}

// Entry point for processing events
func (s *Service) processEvent(ctx context.Context, event BbnEvent) {
	switch event.Category {
	case BlockCategory:
		s.processBbnBlockEvent(ctx, event.Event)
	case TxCategory:
		s.processBbnTxEvent(ctx, event.Event)
	default:
		log.Fatal().Msgf("Unknown event category: %s", event.Category)
	}
}

func (s *Service) processBbnTxEvent(ctx context.Context, event abcitypes.Event) {
	switch EventTypes(event.Type) {
	case EventFinalityProviderCreatedType:
		s.processNewFinalityProviderEvent(ctx, event)
	case EventFinalityProviderEditedType:
		s.processFinalityProviderEditedEvent(ctx, event)

	}
}

func (s *Service) processBbnBlockEvent(ctx context.Context, event abcitypes.Event) {
	switch EventTypes(event.Type) {
	case EventFinalityProviderStateChangeType:
		s.processFinalityProviderStateChangeEvent(ctx, event)
	}
}

func parseEvent[T any](
	expectedType EventTypes,
	event abcitypes.Event,
) (*T, *types.Error) {
	if EventTypes(event.Type) != expectedType {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Sprintf(
				"unexpected event type: %s received when processing %s",
				event.Type,
				expectedType,
			),
		)
	}
	if len(event.Attributes) == 0 {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Sprintf(
				"no attributes found in the %s event",
				expectedType,
			),
		)
	}

	// Create a map to store the attributes
	attributeMap := make(map[string]string)

	// Populate the attribute map from the event's attributes
	for _, attr := range event.Attributes {
		attributeMap[attr.Key] = attr.Value
	}

	// Marshal the attributeMap into JSON
	attrJSON, err := json.Marshal(attributeMap)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to marshal attributes into JSON: %w", err),
		)
	}

	// Unmarshal the JSON into the T struct
	var evt T
	err = json.Unmarshal(attrJSON, &evt)
	if err != nil {
		return nil, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to unmarshal attributes into %T: %w", evt, err),
		)
	}

	return &evt, nil
}
