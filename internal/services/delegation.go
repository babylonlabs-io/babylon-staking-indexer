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
	EventBTCDelegationCreated                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationCreated"
	EventCovenantQuorumReached               EventTypes = "babylon.btcstaking.v1.EventCovenantQuorumReached"
	EventBTCDelegationInclusionProofReceived EventTypes = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventTypes = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationExpired"
)

func (s *Service) processNewBTCDelegationEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	newDelegation, err := parseEvent[*bbntypes.EventBTCDelegationCreated](
		EventBTCDelegationCreated, event,
	)
	if err != nil {
		return err
	}

	if err := s.validateBTCDelegationCreatedEvent(ctx, newDelegation); err != nil {
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

	if err := s.validateCovenantQuorumReachedEvent(ctx, covenantQuorumReachedEvent); err != nil {
		return err
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

	if err := s.validateBTCDelegationInclusionProofReceivedEvent(ctx, inclusionProofEvent); err != nil {
		return err
	}

	if err := s.db.UpdateBTCDelegationDetails(
		ctx, inclusionProofEvent.StakingTxHash, model.FromEventBTCDelegationInclusionProofReceived(inclusionProofEvent),
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

	if err := s.validateBTCDelegationUnbondedEarlyEvent(ctx, unbondedEarlyEvent); err != nil {
		return err
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, unbondedEarlyEvent.StakingTxHash, types.StateUnbonding,
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

	if err := s.validateBTCDelegationExpiredEvent(ctx, expiredEvent); err != nil {
		return err
	}

	delegation, err2 := s.db.GetBTCDelegationByStakingTxHash(ctx, expiredEvent.StakingTxHash)
	if err2 != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err2),
		)
	}
	if err := s.db.SaveNewTimeLockExpire(
		ctx, delegation.StakingTxHashHex, delegation.EndHeight, types.ExpiredTxType.String(),
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save timelock expire: %w", err),
		)
	}

	if err := s.db.UpdateBTCDelegationState(
		ctx, expiredEvent.StakingTxHash, types.StateUnbonding,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	return nil
}

func (s *Service) validateBTCDelegationCreatedEvent(ctx context.Context, event *bbntypes.EventBTCDelegationCreated) *types.Error {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"new BTC delegation event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bbntypes.BTCDelegationStatus_PENDING.String() {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: expected PENDING, got %s", event.NewState),
		)
	}

	return nil
}

func (s *Service) validateCovenantQuorumReachedEvent(ctx context.Context, event *bbntypes.EventCovenantQuorumReached) *types.Error {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"covenant quorum reached event missing staking tx hash",
		)
	}

	// Fetch the current delegation state from the database
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err),
		)
	}

	// Check if the previous state is PENDING
	if delegation.State != types.StatePending {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid state transition: current state is %s, expected PENDING", delegation.State),
		)
	}

	// Check for valid state transitions
	switch event.NewState {
	case bbntypes.BTCDelegationStatus_VERIFIED.String():
		// This will only happen if the staker is following the new pre-approval flow.
		// For more info read https://github.com/babylonlabs-io/pm/blob/main/rfc/rfc-008-staking-transaction-pre-approval.md#handling-of-the-modified--msgcreatebtcdelegation-message

		// Delegation should not have the inclusion proof yet
		if delegation.HasInclusionProof() {
			return types.NewValidationFailedError(
				fmt.Errorf("inclusion proof already received for BTC delegation: %s", event.StakingTxHash),
			)
		}
	case bbntypes.BTCDelegationStatus_ACTIVE.String():
		// This will happen if the inclusion proof is received in MsgCreateBTCDelegation, i.e the staker is following the old flow

		// Delegation should have the inclusion proof
		if !delegation.HasInclusionProof() {
			return types.NewValidationFailedError(
				fmt.Errorf("inclusion proof not received for BTC delegation: %s", event.StakingTxHash),
			)
		}
	default:
		return types.NewValidationFailedError(
			fmt.Errorf("unexpected delegation state from Babylon: %s", event.NewState),
		)
	}

	return nil
}

func (s *Service) validateBTCDelegationInclusionProofReceivedEvent(ctx context.Context, event *bbntypes.EventBTCDelegationInclusionProofReceived) *types.Error {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"inclusion proof received event missing staking tx hash",
		)
	}

	// Fetch the current delegation state from the database
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err),
		)
	}

	// Delegation should not have the inclusion proof yet
	// After this event is processed, the inclusion proof will be set
	if delegation.HasInclusionProof() {
		return types.NewValidationFailedError(
			fmt.Errorf("inclusion proof already received for BTC delegation: %s", event.StakingTxHash),
		)
	}

	// Check for valid state transitions
	switch event.NewState {
	case bbntypes.BTCDelegationStatus_ACTIVE.String():
		// This will only happen if the staker is following the new pre-approval flow.
		// For more info read https://github.com/babylonlabs-io/pm/blob/main/rfc/rfc-008-staking-transaction-pre-approval.md#handling-of-the-modified--msgcreatebtcdelegation-message

		// Delegation should be in VERIFIED state
		if delegation.State != types.StateVerified {
			return types.NewValidationFailedError(
				fmt.Errorf("invalid state transition to ACTIVE: current state is %s, expected VERIFIED", delegation.State),
			)
		}
	case bbntypes.BTCDelegationStatus_PENDING.String():
		// This will happen if the inclusion proof is received in MsgCreateBTCDelegation, i.e the staker is following the old flow

		// Delegation should be in PENDING state
		if delegation.State != types.StatePending {
			return types.NewValidationFailedError(
				fmt.Errorf("invalid state transition to PENDING: current state is %s, expected PENDING", delegation.State),
			)
		}
	default:
		return types.NewValidationFailedError(
			fmt.Errorf("unexpected delegation state from Babylon: %s", event.NewState),
		)
	}

	return nil
}

func (s *Service) validateBTCDelegationUnbondedEarlyEvent(ctx context.Context, event *bbntypes.EventBTCDelgationUnbondedEarly) *types.Error {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"unbonded early event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bbntypes.BTCDelegationStatus_UNBONDED.String() {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: expected UNBONDED, got %s", event.NewState),
		)
	}

	// Fetch the current delegation state from the database
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err),
		)
	}

	// Check if the previous state is ACTIVE
	if delegation.State != types.StateActive {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid state transition: current state is %s, expected ACTIVE", delegation.State),
		)
	}

	return nil
}

func (s *Service) validateBTCDelegationExpiredEvent(ctx context.Context, event *bbntypes.EventBTCDelegationExpired) *types.Error {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"expired event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bbntypes.BTCDelegationStatus_UNBONDED.String() {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid delegation state from Babylon: expected UNBONDED, got %s", event.NewState),
		)
	}

	// Fetch the current delegation state from the database
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHash)
	if err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err),
		)
	}

	// Check if the previous state is ACTIVE
	if delegation.State != types.StateActive {
		return types.NewValidationFailedError(
			fmt.Errorf("invalid state transition: current state is %s, expected ACTIVE", delegation.State),
		)
	}

	return nil
}
