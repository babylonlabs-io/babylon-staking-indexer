package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
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
func (s *Service) StartBbnEventProcessor(ctx context.Context) {
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
	// Note: We no longer need to check for the event category here. We can directly
	// process the event based on its type.
	bbnEvent := event.Event
	// log.Debug().Str("event_type", bbnEvent.Type).Msg("Processing event")
	switch EventTypes(bbnEvent.Type) {
	case EventFinalityProviderCreatedType:
		log.Debug().Msg("Processing new finality provider event")
		s.processNewFinalityProviderEvent(ctx, bbnEvent)
	case EventBTCDelegationCreated:
		log.Debug().Msg("Processing new BTC delegation event")
		s.processNewBTCDelegationEvent(ctx, bbnEvent)
	case EventFinalityProviderEditedType:
		log.Debug().Msg("Processing finality provider edited event")
		s.processFinalityProviderEditedEvent(ctx, bbnEvent)
	case EventFinalityProviderStatusChange:
		log.Debug().Msg("Processing finality provider status change event")
		s.processFinalityProviderStateChangeEvent(ctx, bbnEvent)
	}
}

func parseEvent[T proto.Message](
	expectedType EventTypes,
	event abcitypes.Event,
) (T, *types.Error) {
	var result T

	if EventTypes(event.Type) != expectedType {
		return result, types.NewErrorWithMsg(
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
		return result, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Sprintf(
				"no attributes found in the %s event",
				expectedType,
			),
		)
	}

	// Use the SDK's ParseTypedEvent function
	parsedEvent, err := sdk.ParseTypedEvent(event)
	if err != nil {
		return result, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse typed event: %w", err),
		)
	}

	// Log the parsed event
	log.Debug().
		Interface("parsed_event", parsedEvent).
		Msg("Parsed event details")

	evtType := proto.MessageName(parsedEvent)
	log.Debug().Str("event_type", evtType).Msg("parsed event type")

	// Check if the parsed event is of the expected type
	// if reflect.TypeOf(parsedEvent) != reflect.TypeOf(result) {
	// 	return nil, types.NewError(
	// 		http.StatusInternalServerError,
	// 		types.InternalServiceError,
	// 		fmt.Errorf("parsed event type %T does not match expected type %T", parsedEvent, result),
	// 	)
	// }

	// Create a map to store the attributes
	// attributeMap := make(map[string]string)

	// // Populate the attribute map from the event's attributes
	// for _, attr := range event.Attributes {
	// 	// Unescape the attribute value
	// 	attributeMap[attr.Key] = utils.SafeUnescape(attr.Value)
	// }

	// log.Debug().Interface("attributeMap", attributeMap).Msg("attributeMap")

	// Marshal the attributeMap into JSON
	// attrJSON, err := json.Marshal(attributeMap)
	// if err != nil {
	// 	return nil, types.NewError(
	// 		http.StatusInternalServerError,
	// 		types.InternalServiceError,
	// 		fmt.Errorf("failed to marshal attributes into JSON: %w", err),
	// 	)
	// }

	// Unmarshal the JSON into the T struct
	// var evt T
	// err = json.Unmarshal(attrJSON, &evt)
	// if err != nil {
	// 	return nil, types.NewError(
	// 		http.StatusInternalServerError,
	// 		types.InternalServiceError,
	// 		fmt.Errorf("failed to unmarshal attributes into %T: %w", evt, err),
	// 	)
	// }

	// Type assert the parsed event to the expected type
	result, ok := parsedEvent.(T)
	if !ok {
		return result, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to assert parsed event to type %T", result),
		)
	}

	return result, nil
}
