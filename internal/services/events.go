package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/rs/zerolog/log"
)

type EventTypes string

type EventCategory string

func (e EventTypes) String() string {
	return string(e)
}

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
func (s *Service) processEvent(
	ctx context.Context,
	event BbnEvent,
	blockHeight int64,
) *types.Error {
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
		err = s.processFinalityProviderStateChangeEvent(ctx, bbnEvent)
	case EventBTCDelegationCreated:
		log.Debug().Msg("Processing new BTC delegation event")
		err = s.processNewBTCDelegationEvent(ctx, bbnEvent, blockHeight)
	case EventCovenantQuorumReached:
		log.Debug().Msg("Processing covenant quorum reached event")
		err = s.processCovenantQuorumReachedEvent(ctx, bbnEvent)
	case EventCovenantSignatureReceived:
		log.Debug().Msg("Processing covenant signature received event")
		err = s.processCovenantSignatureReceivedEvent(ctx, bbnEvent)
	case EventBTCDelegationInclusionProofReceived:
		log.Debug().Msg("Processing BTC delegation inclusion proof received event")
		err = s.processBTCDelegationInclusionProofReceivedEvent(ctx, bbnEvent)
	case EventBTCDelgationUnbondedEarly:
		log.Debug().Msg("Processing BTC delegation unbonded early event")
		err = s.processBTCDelegationUnbondedEarlyEvent(ctx, bbnEvent)
	case EventBTCDelegationExpired:
		log.Debug().Msg("Processing BTC delegation expired event")
		err = s.processBTCDelegationExpiredEvent(ctx, bbnEvent)
	case EventSlashedFinalityProvider:
		log.Debug().Msg("Processing slashed finality provider event")
		err = s.processSlashedFinalityProviderEvent(ctx, bbnEvent)
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

	// Sanitize the event attributes before parsing
	sanitizedEvent := sanitizeEvent(event)

	// Use the SDK's ParseTypedEvent function
	protoMsg, err := sdk.ParseTypedEvent(sanitizedEvent)
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

func (s *Service) validateBTCDelegationCreatedEvent(event *bstypes.EventBTCDelegationCreated) *types.Error {
	// Check if the staking tx hex is present
	if event.StakingTxHex == "" {
		return types.NewValidationFailedError(
			fmt.Errorf("new BTC delegation event missing staking tx hex"),
		)
	}

	if event.StakingOutputIndex == "" {
		return types.NewValidationFailedError(
			fmt.Errorf("new BTC delegation event missing staking output index"),
		)
	}

	// Validate the event state
	if event.NewState != bstypes.BTCDelegationStatus_PENDING.String() {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: expected PENDING, got %s", event.NewState),
		)
	}

	return nil
}

func (s *Service) validateCovenantQuorumReachedEvent(ctx context.Context, event *bstypes.EventCovenantQuorumReached) (bool, *types.Error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"covenant quorum reached event missing staking tx hash",
		)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	// Retrieve the qualified states for the intended transition
	qualifiedStates := types.QualifiedStatesForCovenantQuorumReached(event.NewState)
	if qualifiedStates == nil {
		return false, types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: %s", event.NewState),
		)
	}

	// Check if the current state is qualified for the transition
	if !utils.Contains(qualifiedStates, delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Str("currentState", delegation.State.String()).
			Str("newState", event.NewState).
			Msg("Ignoring EventCovenantQuorumReached because current state is not qualified for transition")
		return false, nil // Ignore the event silently
	}

	if event.NewState == bstypes.BTCDelegationStatus_VERIFIED.String() {
		// This will only happen if the staker is following the new pre-approval flow.
		// For more info read https://github.com/babylonlabs-io/pm/blob/main/rfc/rfc-008-staking-transaction-pre-approval.md#handling-of-the-modified--msgcreatebtcdelegation-message

		// Delegation should not have the inclusion proof yet
		if delegation.HasInclusionProof() {
			log.Debug().
				Str("stakingTxHashHex", event.StakingTxHash).
				Str("currentState", delegation.State.String()).
				Str("newState", event.NewState).
				Msg("Ignoring EventCovenantQuorumReached because inclusion proof already received")
			return false, nil
		}
	} else if event.NewState == bstypes.BTCDelegationStatus_ACTIVE.String() {
		// This will happen if the inclusion proof is received in MsgCreateBTCDelegation, i.e the staker is following the old flow

		// Delegation should have the inclusion proof
		if !delegation.HasInclusionProof() {
			log.Debug().
				Str("stakingTxHashHex", event.StakingTxHash).
				Str("currentState", delegation.State.String()).
				Str("newState", event.NewState).
				Msg("Ignoring EventCovenantQuorumReached because inclusion proof not received")
			return false, nil
		}
	}

	return true, nil
}

func (s *Service) validateBTCDelegationInclusionProofReceivedEvent(ctx context.Context, event *bstypes.EventBTCDelegationInclusionProofReceived) (bool, *types.Error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"inclusion proof received event missing staking tx hash",
		)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	// Retrieve the qualified states for the intended transition
	qualifiedStates := types.QualifiedStatesForInclusionProofReceived(event.NewState)
	if qualifiedStates == nil {
		return false, types.NewValidationFailedError(
			fmt.Errorf("no qualified states defined for new state: %s", event.NewState),
		)
	}

	// Check if the current state is qualified for the transition
	if !utils.Contains(qualifiedStates, delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Str("currentState", delegation.State.String()).
			Str("newState", event.NewState).
			Msg("Ignoring EventBTCDelegationInclusionProofReceived because current state is not qualified for transition")
		return false, nil
	}

	// Delegation should not have the inclusion proof yet
	// After this event is processed, the inclusion proof will be set
	if delegation.HasInclusionProof() {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Str("currentState", delegation.State.String()).
			Str("newState", event.NewState).
			Msg("Ignoring EventBTCDelegationInclusionProofReceived because inclusion proof already received")
		return false, nil
	}

	return true, nil
}

func (s *Service) validateBTCDelegationUnbondedEarlyEvent(ctx context.Context, event *bstypes.EventBTCDelgationUnbondedEarly) (bool, *types.Error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"unbonded early event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bstypes.BTCDelegationStatus_UNBONDED.String() {
		return false, types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: expected UNBONDED, got %s", event.NewState),
		)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	// Check if the current state is qualified for the transition
	if !utils.Contains(types.QualifiedStatesForUnbondedEarly(), delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Str("currentState", delegation.State.String()).
			Msg("Ignoring EventBTCDelgationUnbondedEarly because current state is not qualified for transition")
		return false, nil
	}

	return true, nil
}

func (s *Service) validateBTCDelegationExpiredEvent(ctx context.Context, event *bstypes.EventBTCDelegationExpired) (bool, *types.Error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"expired event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bstypes.BTCDelegationStatus_UNBONDED.String() {
		return false, types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: expected UNBONDED, got %s", event.NewState),
		)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	// Check if the current state is qualified for the transition
	if !utils.Contains(types.QualifiedStatesForExpired(), delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Str("currentState", delegation.State.String()).
			Msg("Ignoring EventBTCDelegationExpired because current state is not qualified for transition")
		return false, nil
	}

	return true, nil
}

func (s *Service) validateSlashedFinalityProviderEvent(ctx context.Context, event *ftypes.EventSlashedFinalityProvider) (bool, *types.Error) {
	if event.Evidence == nil {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"slashed finality provider event missing evidence",
		)
	}

	_, err := event.Evidence.ExtractBTCSK()
	if err != nil {
		return false, types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to extract BTC SK of the slashed finality provider: %w", err),
		)
	}

	return true, nil
}

func sanitizeEvent(event abcitypes.Event) abcitypes.Event {
	sanitizedAttrs := make([]abcitypes.EventAttribute, len(event.Attributes))
	for i, attr := range event.Attributes {
		// Remove any extra quotes and ensure proper JSON formatting
		value := strings.Trim(attr.Value, "\"")
		// If the value isn't already a JSON value (object, array, or quoted string),
		// wrap it in quotes
		if !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "[") {
			value = fmt.Sprintf("\"%s\"", value)
		}

		sanitizedAttrs[i] = abcitypes.EventAttribute{
			Key:   attr.Key,
			Value: value,
			Index: attr.Index,
		}
	}

	return abcitypes.Event{
		Type:       event.Type,
		Attributes: sanitizedAttrs,
	}
}
