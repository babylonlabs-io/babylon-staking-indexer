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
	case EventFinalityProviderEditedType:
		log.Debug().Msg("Processing finality provider edited event")
		s.processFinalityProviderEditedEvent(ctx, bbnEvent)
	case EventFinalityProviderStatusChange:
		log.Debug().Msg("Processing finality provider status change event")
		s.processFinalityProviderStateChangeEvent(ctx, bbnEvent)
	case EventBTCDelegationCreated:
		log.Debug().Msg("Processing new BTC delegation event")
		s.processNewBTCDelegationEvent(ctx, bbnEvent)
	case EventBTCDelegationStateUpdate:
		log.Debug().Msg("Processing BTC delegation state update event")
		s.processBTCDelegationStateUpdateEvent(ctx, bbnEvent)
	case EventCovenantQuorumReached:
		log.Debug().Msg("Processing covenant quorum reached event")
		s.processCovenantQuorumReachedEvent(ctx, bbnEvent)
	case EventBTCDelegationInclusionProofReceived:
		log.Debug().Msg("Processing BTC delegation inclusion proof received event")
		s.processBTCDelegationInclusionProofReceivedEvent(ctx, bbnEvent)
	case EventBTCDelgationUnbondedEarly:
		log.Debug().Msg("Processing BTC delegation unbonded early event")
		s.processBTCDelegationUnbondedEarlyEvent(ctx, bbnEvent)
	case EventBTCDelegationExpired:
		log.Debug().Msg("Processing BTC delegation expired event")
		s.processBTCDelegationExpiredEvent(ctx, bbnEvent)
	}
}

func parseEvent[T proto.Message](
	expectedType EventTypes,
	event abcitypes.Event,
) (T, *types.Error) {
	var result T

	// Check if the event type matches the expected type
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

	// Check if the event has attributes
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
	protoMsg, err := sdk.ParseTypedEvent(event)
	if err != nil {
		return result, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse typed event: %w", err),
		)
	}

	// Type assertion to ensure we have the correct concrete type
	concreteMsg, ok := protoMsg.(T)
	if !ok {
		return result, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("parsed event type %T does not match expected type %T", protoMsg, result),
		)
	}

	return concreteMsg, nil
}
