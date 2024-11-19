package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
)

const (
	EventBTCDelegationCreated                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationCreated"
	EventCovenantQuorumReached               EventTypes = "babylon.btcstaking.v1.EventCovenantQuorumReached"
	EventBTCDelegationInclusionProofReceived EventTypes = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventTypes = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationExpired"
	EventSlashedFinalityProvider             EventTypes = "babylon.finality.v1.EventSlashedFinalityProvider"
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

	if err := s.validateBTCDelegationCreatedEvent(newDelegation); err != nil {
		return err
	}

	delegationDoc, err := model.FromEventBTCDelegationCreated(newDelegation)
	if err != nil {
		return err
	}

	if err = s.emitConsumerEvent(ctx, types.StatePending, delegationDoc); err != nil {
		return err
	}

	if dbErr := s.db.SaveNewBTCDelegation(
		ctx, delegationDoc,
	); dbErr != nil {
		if db.IsDuplicateKeyError(dbErr) {
			// BTC delegation already exists, ignore the event
			return nil
		}
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save new BTC delegation: %w", dbErr),
		)
	}

	// TODO: start watching for BTC confirmation if we need PendingBTCConfirmation state

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

	proceed, err := s.validateCovenantQuorumReachedEvent(ctx, covenantQuorumReachedEvent)
	if err != nil {
		return err
	}
	if !proceed {
		// Ignore the event silently
		return nil
	}

	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, covenantQuorumReachedEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}
	newState := types.DelegationState(covenantQuorumReachedEvent.NewState)
	err = s.emitConsumerEvent(ctx, newState, delegation)
	if err != nil {
		return err
	}

	if dbErr := s.db.UpdateBTCDelegationState(
		ctx, covenantQuorumReachedEvent.StakingTxHash, newState,
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", dbErr),
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

	proceed, err := s.validateBTCDelegationInclusionProofReceivedEvent(ctx, inclusionProofEvent)
	if err != nil {
		return err
	}
	if !proceed {
		// Ignore the event silently
		return nil
	}

	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, inclusionProofEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	newState := types.DelegationState(inclusionProofEvent.NewState)
	err = s.emitConsumerEvent(ctx, newState, delegation)
	if err != nil {
		return err
	}

	if dbErr := s.db.UpdateBTCDelegationDetails(
		ctx, inclusionProofEvent.StakingTxHash, model.FromEventBTCDelegationInclusionProofReceived(inclusionProofEvent),
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", dbErr),
		)
	}

	return nil
}

func (s *Service) processBTCDelegationUnbondedEarlyEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	unbondedEarlyEvent, err := parseEvent[*bbntypes.EventBTCDelgationUnbondedEarly](
		EventBTCDelgationUnbondedEarly,
		event,
	)
	if err != nil {
		return err
	}

	shouldProcess, err := s.validateBTCDelegationUnbondedEarlyEvent(ctx, unbondedEarlyEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
		// Event is valid but should be skipped
		return nil
	}

	// Get delegation details
	delegation, err := s.getDelegationDetails(ctx, unbondedEarlyEvent.StakingTxHash)
	if err != nil {
		return err
	}

	// Emit consumer event
	if err := s.emitConsumerEvent(ctx, types.StateUnbonding, delegation); err != nil {
		return err
	}

	// Handle unbonding process
	if err := s.handleUnbondingProcess(ctx, unbondedEarlyEvent, delegation); err != nil {
		return err
	}

	// Start watching for spend
	if err := s.startWatchingUnbondingSpend(ctx, delegation); err != nil {
		return err
	}

	return nil
}

func (s *Service) processBTCDelegationExpiredEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	expiredEvent, err := parseEvent[*bbntypes.EventBTCDelegationExpired](
		EventBTCDelegationExpired,
		event,
	)
	if err != nil {
		return err
	}

	shouldProcess, err := s.validateBTCDelegationExpiredEvent(ctx, expiredEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
		// Event is valid but should be skipped
		return nil
	}

	// Get delegation details
	delegation, err := s.getDelegationDetails(ctx, expiredEvent.StakingTxHash)
	if err != nil {
		return err
	}

	// Emit consumer event
	if err := s.emitConsumerEvent(ctx, types.StateUnbonding, delegation); err != nil {
		return err
	}

	// Handle expiry process
	if err := s.handleExpiryProcess(ctx, delegation); err != nil {
		return err
	}

	// Start watching for spend
	if err := s.startWatchingStakingSpend(ctx, delegation); err != nil {
		return err
	}

	return nil
}

func (s *Service) processSlashedFinalityProviderEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	slashedFinalityProviderEvent, err := parseEvent[*ftypes.EventSlashedFinalityProvider](
		EventSlashedFinalityProvider,
		event,
	)
	if err != nil {
		return err
	}

	shouldProcess, err := s.validateSlashedFinalityProviderEvent(ctx, slashedFinalityProviderEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
		// Event is valid but should be skipped
		return nil
	}

	evidence := slashedFinalityProviderEvent.Evidence
	fpBTCPKHex := evidence.FpBtcPk.MarshalHex()

	if dbErr := s.db.UpdateDelegationsStateByFinalityProvider(
		ctx, fpBTCPKHex, types.StateSlashed,
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", dbErr),
		)
	}

	// TODO: babylon needs to emit slashing tx
	// so indexer can start watching for slashing spend
	// to identify if staker has withdrawn after slashing

	return nil
}
