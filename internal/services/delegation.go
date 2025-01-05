package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
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

	// Emit event and register spend notification
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, covenantQuorumReachedEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}

	newState := types.DelegationState(covenantQuorumReachedEvent.NewState)
	if newState == types.StateActive {
		log.Debug().
			Str("staking_tx", covenantQuorumReachedEvent.StakingTxHash).
			Str("staking_start_height", strconv.FormatUint(uint64(delegation.StartHeight), 10)).
			Str("event_type", EventCovenantQuorumReached.String()).
			Msg("handling active state")

		err = s.emitActiveDelegationEvent(
			ctx,
			delegation.StakingTxHashHex,
			delegation.StakerBtcPkHex,
			delegation.FinalityProviderBtcPksHex,
			delegation.StakingAmount,
		)
		if err != nil {
			return err
		}

		if err := s.registerStakingSpendNotification(
			ctx,
			delegation.StakingTxHashHex,
			delegation.StakingTxHex,
			delegation.StakingOutputIdx,
			delegation.StartHeight,
		); err != nil {
			return err
		}
	}

	// Update delegation state
	if dbErr := s.db.UpdateBTCDelegationState(
		ctx,
		covenantQuorumReachedEvent.StakingTxHash,
		types.QualifiedStatesForCovenantQuorumReached(covenantQuorumReachedEvent.NewState),
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

	// Emit event and register spend notification
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, inclusionProofEvent.StakingTxHash)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
		)
	}
	newState := types.DelegationState(inclusionProofEvent.NewState)
	if newState == types.StateActive {
		stakingStartHeight, _ := strconv.ParseUint(inclusionProofEvent.StartHeight, 10, 32)

		log.Debug().
			Str("staking_tx", inclusionProofEvent.StakingTxHash).
			Str("staking_start_height", inclusionProofEvent.StartHeight).
			Str("event_type", EventBTCDelegationInclusionProofReceived.String()).
			Msg("handling active state")

		err = s.emitActiveDelegationEvent(
			ctx,
			inclusionProofEvent.StakingTxHash,
			delegation.StakerBtcPkHex,
			delegation.FinalityProviderBtcPksHex,
			delegation.StakingAmount,
		)
		if err != nil {
			return err
		}

		if err := s.registerStakingSpendNotification(ctx,
			delegation.StakingTxHashHex,
			delegation.StakingTxHex,
			delegation.StakingOutputIdx,
			uint32(stakingStartHeight),
		); err != nil {
			return err
		}
	}

	// Update delegation details
	if dbErr := s.db.UpdateBTCDelegationDetails(
		ctx,
		inclusionProofEvent.StakingTxHash,
		model.FromEventBTCDelegationInclusionProofReceived(inclusionProofEvent),
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation details: %w", dbErr),
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

	shouldProcess, shouldEmitEvent, err := s.validateBTCDelegationUnbondedEarlyEvent(ctx, unbondedEarlyEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
		// Event is valid but should be skipped
		if shouldEmitEvent {
			log.Debug().
				Str("staking_tx", unbondedEarlyEvent.StakingTxHash).
				Str("event_type", EventBTCDelgationUnbondedEarly.String()).
				Msg("skip processing but emit unbonding event")

			delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, unbondedEarlyEvent.StakingTxHash)
			if dbErr != nil {
				return types.NewError(
					http.StatusInternalServerError,
					types.InternalServiceError,
					fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
				)
			}

			// Emit consumer event
			if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
				return err
			}
		}
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
	if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
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

	log.Debug().
		Str("staking_tx", unbondedEarlyEvent.StakingTxHash).
		Str("new_state", types.StateUnbonding.String()).
		Str("early_unbonding_start_height", unbondedEarlyEvent.StartHeight).
		Str("unbonding_time", strconv.FormatUint(uint64(delegation.UnbondingTime), 10)).
		Str("unbonding_expire_height", strconv.FormatUint(uint64(unbondingExpireHeight), 10)).
		Str("sub_state", subState.String()).
		Str("event_type", EventBTCDelgationUnbondedEarly.String()).
		Msg("updating delegation state")

	// Update delegation state
	if err := s.db.UpdateBTCDelegationState(
		ctx,
		unbondedEarlyEvent.StakingTxHash,
		types.QualifiedStatesForUnbondedEarly(),
		types.StateUnbonding,
		&subState,
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
		EventBTCDelegationExpired,
		event,
	)
	if err != nil {
		return err
	}

	shouldProcess, shouldEmitEvent, err := s.validateBTCDelegationExpiredEvent(ctx, expiredEvent)
	if err != nil {
		return err
	}
	if !shouldProcess {
		// Event is valid but should be skipped
		if shouldEmitEvent {
			log.Debug().
				Str("staking_tx", expiredEvent.StakingTxHash).
				Str("event_type", EventBTCDelegationExpired.String()).
				Msg("skip processing but emit unbonding event")

			delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, expiredEvent.StakingTxHash)
			if dbErr != nil {
				return types.NewError(
					http.StatusInternalServerError,
					types.InternalServiceError,
					fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr),
				)
			}

			// Emit consumer event
			if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
				return err
			}
		}
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
	if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
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
		types.QualifiedStatesForExpired(),
		types.StateUnbonding,
		&subState,
	); err != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", err),
		)
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
		ctx, fpBTCPKHex, types.QualifiedStatesForSlashedDelegation(), types.StateSlashed,
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", dbErr),
		)
	}

	delegations, dbErr := s.db.GetDelegationsByFinalityProvider(ctx, fpBTCPKHex)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get BTC delegations by finality provider: %w", dbErr),
		)
	}

	// TODO: ideally indexer should simply emit the slashed FP
	// queue handlers should handle the rest
	for _, delegation := range delegations {
		if !delegation.HasInclusionProof() {
			// If the delegation was never active/has no inclusion proof
			// no need to emit the event, as it doesn't contribute to stats
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("event_type", EventSlashedFinalityProvider.String()).
				Str("current_state", delegation.State.String()).
				Str("reason", "missing_inclusion_proof").
				Msg("skipping slashed delegation event")
			continue
		}

		if !utils.Contains(types.QualifiedStatesForSlashedDelegation(), delegation.State) {
			// If the current state is not qualified, no need to emit the event
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("event_type", EventSlashedFinalityProvider.String()).
				Str("current_state", delegation.State.String()).
				Str("reason", "not_qualified").
				Msg("skipped slashed delegation event")
			continue
		}

		if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
			return err
		}
	}

	return nil
}
