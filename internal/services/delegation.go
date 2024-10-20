package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
)

const (
	EventBTCDelegationStateUpdate            EventTypes = "babylon.btcstaking.v1.EventBTCDelegationStateUpdate"
	EventBTCDelegationCreated                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationCreated"
	EventCovenantQuorumReached               EventTypes = "babylon.btcstaking.v1.EventCovenantQuorumReached"
	EventBTCDelegationInclusionProofReceived EventTypes = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventTypes = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationExpired"
)

func (s *Service) processBTCDelegationStateUpdateEvent(ctx context.Context, event abcitypes.Event) *types.Error {
	stateUpdate, err := parseEvent[*bbntypes.EventBTCDelegationStateUpdate](
		EventBTCDelegationStateUpdate, event,
	)
	if err != nil {
		return err
	}

	if err := validateBTCDelegationStateUpdateEvent(stateUpdate); err != nil {
		return err
	}

	// Check if BTC delegation exists
	_, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, stateUpdate.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, stateUpdate.StakingTxHash, types.DelegationState(stateUpdate.NewState),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

func (s *Service) processNewBTCDelegationEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	newDelegation, err := parseEvent[*bbntypes.EventBTCDelegationCreated](
		EventBTCDelegationCreated, event,
	)
	if err != nil {
		return err
	}

	if err := validateBTCDelegationCreatedEvent(newDelegation); err != nil {
		return err
	}
	if err := s.db.SaveNewBTCDelegation(
		ctx, model.FromEventBTCDelegationCreated(newDelegation),
	); err != nil {
		if db.IsDuplicateKeyError(err) {
			// BTC delegation already exists, ignore the event
			return nil
		}
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save new BTC delegation: %w", err),
		)
	}

	return nil
}

func (s *Service) processCovenantQuorumReachedEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	covenantQuorumReachedEvent, err := parseEvent[*bbntypes.EventCovenantQuorumReached](
		EventCovenantQuorumReached, event,
	)
	if err != nil {
		return err
	}

	if err := validateCovenantQuorumReachedEvent(covenantQuorumReachedEvent); err != nil {
		return err
	}

	// Check if BTC delegation exists
	_, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, covenantQuorumReachedEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, covenantQuorumReachedEvent.StakingTxHash, types.DelegationState(covenantQuorumReachedEvent.NewState),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

func (s *Service) processBTCDelegationInclusionProofReceivedEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	inclusionProofEvent, err := parseEvent[*bbntypes.EventBTCDelegationInclusionProofReceived](
		EventBTCDelegationInclusionProofReceived, event,
	)
	if err != nil {
		return err
	}

	if err := validateBTCDelegationInclusionProofReceivedEvent(inclusionProofEvent); err != nil {
		return err
	}

	// Check if BTC delegation exists
	_, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, inclusionProofEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, inclusionProofEvent.StakingTxHash, types.DelegationState(inclusionProofEvent.NewState),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

func (s *Service) processBTCDelegationUnbondedEarlyEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	unbondedEarlyEvent, err := parseEvent[*bbntypes.EventBTCDelgationUnbondedEarly](
		EventBTCDelgationUnbondedEarly, event,
	)
	if err != nil {
		return err
	}

	if err := validateBTCDelegationUnbondedEarlyEvent(unbondedEarlyEvent); err != nil {
		return err
	}

	// Check if BTC delegation exists
	_, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, unbondedEarlyEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, unbondedEarlyEvent.StakingTxHash, types.DelegationState(unbondedEarlyEvent.NewState),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

func (s *Service) processBTCDelegationExpiredEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	expiredEvent, err := parseEvent[*bbntypes.EventBTCDelegationExpired](
		EventBTCDelegationExpired, event,
	)
	if err != nil {
		return err
	}

	if err := validateBTCDelegationExpiredEvent(expiredEvent); err != nil {
		return err
	}

	// Check if BTC delegation exists
	_, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, expiredEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, expiredEvent.StakingTxHash, types.DelegationState(expiredEvent.NewState),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

// You'll need to implement these functions:
func validateBTCDelegationCreatedEvent(event *bbntypes.EventBTCDelegationCreated) *types.Error {
	// Implement validation logic here
	return nil
}

func validateBTCDelegationStateUpdateEvent(event *bbntypes.EventBTCDelegationStateUpdate) *types.Error {
	// Implement validation logic here
	return nil
}

func validateCovenantQuorumReachedEvent(event *bbntypes.EventCovenantQuorumReached) *types.Error {
	// Implement validation logic here
	return nil
}

func validateBTCDelegationInclusionProofReceivedEvent(event *bbntypes.EventBTCDelegationInclusionProofReceived) *types.Error {
	// Implement validation logic here
	return nil
}

func validateBTCDelegationUnbondedEarlyEvent(event *bbntypes.EventBTCDelgationUnbondedEarly) *types.Error {
	// Implement validation logic here
	return nil
}

func validateBTCDelegationExpiredEvent(event *bbntypes.EventBTCDelegationExpired) *types.Error {
	// Implement validation logic here
	return nil
}
