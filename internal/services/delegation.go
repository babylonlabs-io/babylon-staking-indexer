package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
	EventCovenantSignatureReceived           EventTypes = "babylon.btcstaking.v1.EventCovenantSignatureReceived"
	EventBTCDelegationInclusionProofReceived EventTypes = "babylon.btcstaking.v1.EventBTCDelegationInclusionProofReceived"
	EventBTCDelgationUnbondedEarly           EventTypes = "babylon.btcstaking.v1.EventBTCDelgationUnbondedEarly"
	EventBTCDelegationExpired                EventTypes = "babylon.btcstaking.v1.EventBTCDelegationExpired"
	EventSlashedFinalityProvider             EventTypes = "babylon.finality.v1.EventSlashedFinalityProvider"
)

func (s *Service) processNewBTCDelegationEvent(
	ctx context.Context, event abcitypes.Event, bbnBlockHeight int64,
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

	// Get block info to get timestamp
	bbnBlock, bbnErr := s.bbn.GetBlock(ctx, &bbnBlockHeight)
	if bbnErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.ClientRequestError,
			fmt.Errorf("failed to get block: %w", bbnErr),
		)
	}
	bbnBlockTime := bbnBlock.Block.Time.Unix()

	delegationDoc, err := model.FromEventBTCDelegationCreated(newDelegation, bbnBlockHeight, bbnBlockTime)
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

func (s *Service) processCovenantSignatureReceivedEvent(
	ctx context.Context, event abcitypes.Event,
) *types.Error {
	covenantSignatureReceivedEvent, err := parseEvent[*bbntypes.EventCovenantSignatureReceived](
		EventCovenantSignatureReceived, event,
	)
	if err != nil {
		return err
	}
	stakingTxHash := covenantSignatureReceivedEvent.StakingTxHash
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}
	// Check if the covenant signature already exists, if it does, ignore the event
	for _, signature := range delegation.CovenantUnbondingSignatures {
		if signature.CovenantBtcPkHex == covenantSignatureReceivedEvent.CovenantBtcPkHex {
			return nil
		}
	}
	// Breakdown the covenantSignatureReceivedEvent into individual fields
	covenantBtcPkHex := covenantSignatureReceivedEvent.CovenantBtcPkHex
	signatureHex := covenantSignatureReceivedEvent.CovenantUnbondingSignatureHex

	if dbErr := s.db.SaveBTCDelegationUnbondingCovenantSignature(
		ctx,
		stakingTxHash,
		covenantBtcPkHex,
		signatureHex,
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf(
				"failed to save BTC delegation unbonding covenant signature: %w for staking tx hash %s",
				dbErr, stakingTxHash,
			),
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

	shouldProcess, err := s.validateCovenantQuorumReachedEvent(ctx, covenantQuorumReachedEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
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
		ctx,
		covenantQuorumReachedEvent.StakingTxHash,
		newState,
		nil,
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

	shouldProcess, err := s.validateBTCDelegationInclusionProofReceivedEvent(ctx, inclusionProofEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
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

	if dbErr := s.db.UpdateBTCDelegationState(
		ctx,
		inclusionProofEvent.StakingTxHash,
		newState,
		nil,
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

	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, unbondedEarlyEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	// Emit consumer event
	if err := s.emitConsumerEvent(ctx, types.StateUnbonding, delegation); err != nil {
		return err
	}

	unbondingStartHeight, parseErr := strconv.ParseUint(unbondedEarlyEvent.StartHeight, 10, 32)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse start height: %w", parseErr),
		)
	}

	subState := types.SubStateEarlyUnbonding

	// Save timelock expire
	unbondingExpireHeight := uint32(unbondingStartHeight) + delegation.UnbondingTime
	if err := s.db.SaveNewTimeLockExpire(
		ctx,
		delegation.StakingTxHashHex,
		unbondingExpireHeight,
		subState,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save timelock expire: %w", err),
		)
	}

	// Update delegation state
	if err := s.db.UpdateBTCDelegationState(
		ctx,
		unbondedEarlyEvent.StakingTxHash,
		types.StateUnbonding,
		&subState,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	// Register unbonding spend notification
	if err := s.registerUnbondingSpendNotification(ctx, delegation); err != nil {
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

	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, expiredEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	// Emit consumer event
	if err := s.emitConsumerEvent(ctx, types.StateUnbonding, delegation); err != nil {
		return err
	}

	subState := types.SubStateTimelock

	// Save timelock expire
	if err := s.db.SaveNewTimeLockExpire(
		ctx,
		delegation.StakingTxHashHex,
		delegation.EndHeight,
		subState,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save timelock expire: %w", err),
		)
	}

	// Update delegation state
	if err := s.db.UpdateBTCDelegationState(
		ctx,
		delegation.StakingTxHashHex,
		types.StateUnbonding,
		&subState,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
	}

	// Register spend notification
	if err := s.registerStakingSpendNotification(ctx, delegation); err != nil {
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

	return nil
}
