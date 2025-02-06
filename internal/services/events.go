package services

import (
	"context"
	"fmt"
	"strings"
	"slices"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/rs/zerolog/log"
	"github.com/avast/retry-go/v4"
)

const (
	BlockCategory types.EventCategory = "block"
	TxCategory    types.EventCategory = "tx"

	processEventMaxRetries = 3
)

type BbnEvent struct {
	Category types.EventCategory
	Event    abcitypes.Event
}

func NewBbnEvent(category types.EventCategory, event abcitypes.Event) BbnEvent {
	return BbnEvent{
		Category: category,
		Event:    event,
	}
}

// Entry point for processing events with retries
func (s *Service) processEvent(
	ctx context.Context,
	event BbnEvent,
	blockHeight int64,
) error {
	f := func() error {
		return s.doProcessEvent(ctx, event, blockHeight)
	}

	// by default exponential delay is going to be used
	err := retry.Do(
		f,
		retry.Attempts(processEventMaxRetries),
		retry.Delay(retryInitialDelay),
		retry.MaxDelay(retryMaxAllowedDelay),
	)

	return err
}

func (s *Service) doProcessEvent(
	ctx context.Context,
	event BbnEvent,
	blockHeight int64,
) error {
	// Note: We no longer need to check for the event category here. We can directly
	// process the event based on its type.
	bbnEvent := event.Event

	var err error

	switch types.EventType(bbnEvent.Type) {
	case types.EventFinalityProviderCreatedType:
		log.Debug().Msg("Processing new finality provider event")
		err = s.processNewFinalityProviderEvent(ctx, bbnEvent)
	case types.EventFinalityProviderEditedType:
		log.Debug().Msg("Processing finality provider edited event")
		err = s.processFinalityProviderEditedEvent(ctx, bbnEvent)
	case types.EventFinalityProviderStatusChange:
		log.Debug().Msg("Processing finality provider status change event")
		err = s.processFinalityProviderStateChangeEvent(ctx, bbnEvent)
	case types.EventBTCDelegationCreated:
		log.Debug().Msg("Processing new BTC delegation event")
		err = s.processNewBTCDelegationEvent(ctx, bbnEvent, blockHeight)
	case types.EventCovenantQuorumReached:
		log.Debug().Msg("Processing covenant quorum reached event")
		err = s.processCovenantQuorumReachedEvent(ctx, bbnEvent, blockHeight)
	case types.EventCovenantSignatureReceived:
		log.Debug().Msg("Processing covenant signature received event")
		err = s.processCovenantSignatureReceivedEvent(ctx, bbnEvent)
	case types.EventBTCDelegationInclusionProofReceived:
		log.Debug().Msg("Processing BTC delegation inclusion proof received event")
		err = s.processBTCDelegationInclusionProofReceivedEvent(ctx, bbnEvent, blockHeight)
	case types.EventBTCDelgationUnbondedEarly:
		log.Debug().Msg("Processing BTC delegation unbonded early event")
		err = s.processBTCDelegationUnbondedEarlyEvent(ctx, bbnEvent, blockHeight)
	case types.EventBTCDelegationExpired:
		log.Debug().Msg("Processing BTC delegation expired event")
		err = s.processBTCDelegationExpiredEvent(ctx, bbnEvent, blockHeight)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to process event")
		return err
	}

	return nil
}

func parseEvent[T proto.Message](
	expectedType types.EventType,
	event abcitypes.Event,
) (T, error) {
	var result T

	// Check if the event type matches the expected type
	if types.EventType(event.Type) != expectedType {
		return result, fmt.Errorf(
			"unexpected event type: %s received when processing %s",
			event.Type,
			expectedType,
		)
	}

	// Check if the event has attributes
	if len(event.Attributes) == 0 {
		return result, fmt.Errorf(
			"no attributes found in the %s event",
			expectedType,
		)
	}

	// Sanitize the event attributes before parsing
	sanitizedEvent := sanitizeEvent(event)

	// Use the SDK's ParseTypedEvent function
	protoMsg, err := sdk.ParseTypedEvent(sanitizedEvent)
	if err != nil {
		log.Debug().Interface("raw_event", event).Msg("Raw event data")
		return result, fmt.Errorf("failed to parse typed event: %w", err)
	}

	// Type assertion to ensure we have the correct concrete type
	concreteMsg, ok := protoMsg.(T)
	if !ok {
		return result, fmt.Errorf("parsed event type %T does not match expected type %T", protoMsg, result)
	}

	return concreteMsg, nil
}

func (s *Service) validateBTCDelegationCreatedEvent(event *bstypes.EventBTCDelegationCreated) error {
	// Check if the staking tx hex is present
	if event.StakingTxHex == "" {
		return fmt.Errorf("new BTC delegation event missing staking tx hex")
	}

	if event.StakingOutputIndex == "" {
		return fmt.Errorf("new BTC delegation event missing staking output index")
	}

	// Validate the event state
	if event.NewState != bstypes.BTCDelegationStatus_PENDING.String() {
		return fmt.Errorf("invalid delegation state from Babylon when processing EventBTCDelegationCreated: expected PENDING, got %s", event.NewState)
	}

	return nil
}

func (s *Service) validateCovenantQuorumReachedEvent(ctx context.Context, event *bstypes.EventCovenantQuorumReached) (bool, error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, fmt.Errorf("covenant quorum reached event missing staking tx hash")
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}

	// Retrieve the qualified states for the intended transition
	qualifiedStates := types.QualifiedStatesForCovenantQuorumReached(event.NewState)
	if qualifiedStates == nil {
		return false, fmt.Errorf("invalid delegation state from Babylon: %s", event.NewState)
	}

	// Check if the current state is qualified for the transition
	if !slices.Contains(qualifiedStates, delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Stringer("currentState", delegation.State).
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
				Stringer("currentState", delegation.State).
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
				Stringer("currentState", delegation.State).
				Str("newState", event.NewState).
				Msg("Ignoring EventCovenantQuorumReached because inclusion proof not received")
			return false, nil
		}
	}

	return true, nil
}

func (s *Service) validateBTCDelegationInclusionProofReceivedEvent(ctx context.Context, event *bstypes.EventBTCDelegationInclusionProofReceived) (bool, error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, fmt.Errorf("inclusion proof received event missing staking tx hash")
	}

	// Check if the start height and end height are present
	if event.StartHeight == "" || event.EndHeight == "" {
		return false, fmt.Errorf("inclusion proof received event missing start height or end height")
	}

	// Check if the start height and end height are valid
	_, err := utils.ParseUint32(event.StartHeight)
	if err != nil {
		return false, fmt.Errorf("failed to parse staking start height: %w", err)
	}
	_, err = utils.ParseUint32(event.EndHeight)
	if err != nil {
		return false, fmt.Errorf("failed to parse staking end height: %w", err)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}

	// Retrieve the qualified states for the intended transition
	qualifiedStates := types.QualifiedStatesForInclusionProofReceived(event.NewState)
	if qualifiedStates == nil {
		return false, fmt.Errorf("no qualified states defined for new state: %s", event.NewState)
	}

	// Check if the current state is qualified for the transition
	if !slices.Contains(qualifiedStates, delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Stringer("currentState", delegation.State).
			Str("newState", event.NewState).
			Msg("Ignoring EventBTCDelegationInclusionProofReceived because current state is not qualified for transition")
		return false, nil
	}

	// Delegation should not have the inclusion proof yet
	// After this event is processed, the inclusion proof will be set
	if delegation.HasInclusionProof() {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Stringer("currentState", delegation.State).
			Str("newState", event.NewState).
			Msg("Ignoring EventBTCDelegationInclusionProofReceived because inclusion proof already received")
		return false, nil
	}

	return true, nil
}

func (s *Service) validateBTCDelegationUnbondedEarlyEvent(ctx context.Context, event *bstypes.EventBTCDelgationUnbondedEarly) (bool, error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, fmt.Errorf("unbonded early event missing staking tx hash")
	}

	// Validate the event state
	if event.NewState != bstypes.BTCDelegationStatus_UNBONDED.String() {
		return false, fmt.Errorf("invalid delegation state from Babylon when processing EventBTCDelgationUnbondedEarly: expected UNBONDED, got %s", event.NewState)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}

	// Check if the current state is qualified for the transition
	if !slices.Contains(types.QualifiedStatesForUnbondedEarly(), delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Stringer("currentState", delegation.State).
			Msg("Ignoring EventBTCDelgationUnbondedEarly because current state is not qualified for transition")
		return false, nil
	}

	return true, nil
}

func (s *Service) validateBTCDelegationExpiredEvent(ctx context.Context, event *bstypes.EventBTCDelegationExpired) (bool, error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, fmt.Errorf("expired event missing staking tx hash")
	}

	// Validate the event state
	if event.NewState != bstypes.BTCDelegationStatus_EXPIRED.String() {
		return false, fmt.Errorf("invalid delegation state from Babylon when processing EventBTCDelegationExpired: expected EXPIRED, got %s", event.NewState)
	}

	// Fetch the current delegation state from the database
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if dbErr != nil {
		return false, fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}

	// Check if the current state is qualified for the transition
	if !slices.Contains(types.QualifiedStatesForExpired(), delegation.State) {
		log.Debug().
			Str("stakingTxHashHex", event.StakingTxHash).
			Stringer("currentState", delegation.State).
			Msg("Ignoring EventBTCDelegationExpired because current state is not qualified for transition")
		return false, nil
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
