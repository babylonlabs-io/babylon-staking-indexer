package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

// Entry point for processing events
func (s *Service) processEvent(ctx context.Context, event BbnEvent) *types.Error {
	// Note: We no longer need to check for the event category here. We can directly
	// process the event based on its type.
	bbnEvent := event.Event

	var err *types.Error

	switch EventTypes(bbnEvent.Type) {
	case EventFinalityProviderCreatedType:
		log.Debug().Msg("Processing new finality provider event")
		err = s.processNewFinalityProviderEvent(ctx, bbnEvent)
	case EventFinalityProviderEditedType:
		log.Debug().Msg("Processing finality provider edited event")
		err = s.processFinalityProviderEditedEvent(ctx, bbnEvent)
	case EventFinalityProviderStatusChange:
		log.Debug().Msg("Processing finality provider status change event")
		// TODO: fix error from this event
		// https://github.com/babylonlabs-io/babylon-staking-indexer/issues/24
		s.processFinalityProviderStateChangeEvent(ctx, bbnEvent)
	case EventBTCDelegationCreated:
		log.Debug().Msg("Processing new BTC delegation event")
		err = s.processNewBTCDelegationEvent(ctx, bbnEvent)
	case EventCovenantQuorumReached:
		log.Debug().Msg("Processing covenant quorum reached event")
		err = s.processCovenantQuorumReachedEvent(ctx, bbnEvent)
	case EventBTCDelegationInclusionProofReceived:
		log.Debug().Msg("Processing BTC delegation inclusion proof received event")
		err = s.processBTCDelegationInclusionProofReceivedEvent(ctx, bbnEvent)
	case EventBTCDelgationUnbondedEarly:
		log.Debug().Msg("Processing BTC delegation unbonded early event")
		err = s.processBTCDelegationUnbondedEarlyEvent(ctx, bbnEvent)
	case EventBTCDelegationExpired:
		log.Debug().Msg("Processing BTC delegation expired event")
		err = s.processBTCDelegationExpiredEvent(ctx, bbnEvent)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to process event")
		return err
	}

	return nil
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
		log.Debug().Interface("raw_event", event).Msg("Raw event data")
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

func (s *Service) parseAndValidateUnbondedEarlyEvent(
	ctx context.Context,
	event abcitypes.Event,
) (*bbntypes.EventBTCDelgationUnbondedEarly, *types.Error) {
	// Parse event
	unbondedEarlyEvent, err := parseEvent[*bbntypes.EventBTCDelgationUnbondedEarly](
		EventBTCDelgationUnbondedEarly,
		event,
	)
	if err != nil {
		return nil, err
	}

	// Validate event
	proceed, err := s.validateBTCDelegationUnbondedEarlyEvent(ctx, unbondedEarlyEvent)
	if err != nil {
		return nil, err
	}
	if !proceed {
		return nil, nil
	}

	return unbondedEarlyEvent, nil
}
