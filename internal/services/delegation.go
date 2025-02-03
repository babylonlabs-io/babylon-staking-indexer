package services

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) processNewBTCDelegationEvent(
	ctx context.Context, event abcitypes.Event, bbnBlockHeight int64,
) error {
	newDelegation, err := parseEvent[*bbntypes.EventBTCDelegationCreated](
		types.EventBTCDelegationCreated, event,
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
		return fmt.Errorf("failed to get block: %w", bbnErr)
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
		return fmt.Errorf("failed to save new BTC delegation: %w", dbErr)
	}

	return nil
}

func (s *Service) processCovenantSignatureReceivedEvent(
	ctx context.Context, event abcitypes.Event,
) error {
	covenantSignatureReceivedEvent, err := parseEvent[*bbntypes.EventCovenantSignatureReceived](
		types.EventCovenantSignatureReceived, event,
	)
	if err != nil {
		return err
	}
	stakingTxHash := covenantSignatureReceivedEvent.StakingTxHash
	delegation, dbErr := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if dbErr != nil {
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
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
		return fmt.Errorf(
			"failed to save BTC delegation unbonding covenant signature: %w for staking tx hash %s",
			dbErr, stakingTxHash,
		)
	}

	return nil
}

func (s *Service) processCovenantQuorumReachedEvent(
	ctx context.Context, event abcitypes.Event, bbnBlockHeight int64,
) error {
	covenantQuorumReachedEvent, err := parseEvent[*bbntypes.EventCovenantQuorumReached](
		types.EventCovenantQuorumReached, event,
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
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}

	newState := types.DelegationState(covenantQuorumReachedEvent.NewState)
	if newState == types.StateActive {
		log.Debug().
			Str("staking_tx", covenantQuorumReachedEvent.StakingTxHash).
			Uint32("staking_start_height", delegation.StartHeight).
			Stringer("event_type", types.EventCovenantQuorumReached).
			Msg("handling active state")

		err = s.emitActiveDelegationEvent(
			ctx,
			delegation,
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
		db.WithBbnHeight(bbnBlockHeight),
		db.WithBbnEventType(types.EventCovenantQuorumReached),
	); dbErr != nil {
		return fmt.Errorf("failed to update BTC delegation state: %w", dbErr)
	}

	return nil
}

func (s *Service) processBTCDelegationInclusionProofReceivedEvent(
	ctx context.Context, event abcitypes.Event, bbnBlockHeight int64,
) error {
	inclusionProofEvent, err := parseEvent[*bbntypes.EventBTCDelegationInclusionProofReceived](
		types.EventBTCDelegationInclusionProofReceived, event,
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
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}
	newState := types.DelegationState(inclusionProofEvent.NewState)
	if newState == types.StateActive {
		stakingStartHeight, _ := utils.ParseUint32(inclusionProofEvent.StartHeight)

		log.Debug().
			Str("staking_tx", inclusionProofEvent.StakingTxHash).
			Str("staking_start_height", inclusionProofEvent.StartHeight).
			Stringer("event_type", types.EventBTCDelegationInclusionProofReceived).
			Msg("handling active state")

		err = s.emitActiveDelegationEvent(
			ctx,
			delegation,
		)
		if err != nil {
			return err
		}

		if err := s.registerStakingSpendNotification(ctx,
			delegation.StakingTxHashHex,
			delegation.StakingTxHex,
			delegation.StakingOutputIdx,
			stakingStartHeight,
		); err != nil {
			return err
		}
	}

	stakingStartHeight, _ := utils.ParseUint32(inclusionProofEvent.StartHeight)
	stakingEndHeight, _ := utils.ParseUint32(inclusionProofEvent.EndHeight)
	stakingBtcTimestamp, err := s.btc.GetBlockTimestamp(stakingStartHeight)
	if err != nil {
		return fmt.Errorf("failed to get block timestamp: %w", err)
	}

	// Note on state history:
	// In the old staking flow, EventBTCDelegationInclusionProofReceived emits a PENDING state.
	// This creates duplicate PENDING entries in state_history:
	// 1. First PENDING: From EventBTCDelegationCreated
	// 2. Second PENDING: From EventBTCDelegationInclusionProofReceived
	//
	// This duplicate entry is expected and maintains consistency with Babylon's state transitions.
	if dbErr := s.db.UpdateBTCDelegationState(
		ctx,
		inclusionProofEvent.StakingTxHash,
		types.QualifiedStatesForInclusionProofReceived(inclusionProofEvent.NewState),
		newState,
		db.WithBbnHeight(bbnBlockHeight),
		db.WithStakingStartHeight(stakingStartHeight),
		db.WithStakingEndHeight(stakingEndHeight),
		db.WithStakingBTCTimestamp(stakingBtcTimestamp),
		db.WithBbnEventType(types.EventBTCDelegationInclusionProofReceived),
	); dbErr != nil {
		return fmt.Errorf("failed to update BTC delegation state: %w", dbErr)
	}

	return nil
}

// TODO: Indexer doesn't need to intercept processBTCDelegationUnbondedEarlyEvent
// as the unbonding tx will be discovered by the btc notifier
// we are keeping it for now to avoid breaking changes, but if the btc notifier has already identified
// then this event will be silently ignored with help of validateBTCDelegationUnbondedEarlyEvent
func (s *Service) processBTCDelegationUnbondedEarlyEvent(
	ctx context.Context, event abcitypes.Event, bbnBlockHeight int64,
) error {
	unbondedEarlyEvent, err := parseEvent[*bbntypes.EventBTCDelgationUnbondedEarly](
		types.EventBTCDelgationUnbondedEarly,
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
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
	}

	// Emit consumer event
	if err := s.emitUnbondingDelegationEvent(ctx, delegation); err != nil {
		return err
	}

	unbondingStartHeight, parseErr := utils.ParseUint32(unbondedEarlyEvent.StartHeight)
	if parseErr != nil {
		return fmt.Errorf("failed to parse start height: %w", parseErr)
	}

	unbondingBtcTimestamp, err := s.btc.GetBlockTimestamp(unbondingStartHeight)
	if err != nil {
		return fmt.Errorf("failed to get block timestamp: %w", err)
	}

	subState := types.SubStateEarlyUnbonding

	// Save timelock expire
	unbondingExpireHeight := unbondingStartHeight + delegation.UnbondingTime
	if err := s.db.SaveNewTimeLockExpire(
		ctx,
		delegation.StakingTxHashHex,
		unbondingExpireHeight,
		subState,
	); err != nil {
		return fmt.Errorf("failed to save timelock expire: %w", err)
	}

	log.Debug().
		Str("staking_tx", unbondedEarlyEvent.StakingTxHash).
		Stringer("current_state", delegation.State).
		Stringer("new_state", types.StateUnbonding).
		Str("early_unbonding_start_height", unbondedEarlyEvent.StartHeight).
		Uint32("unbonding_time", delegation.UnbondingTime).
		Uint32("unbonding_expire_height", unbondingExpireHeight).
		Stringer("sub_state", subState).
		Stringer("event_type", types.EventBTCDelgationUnbondedEarly).
		Msg("updating delegation state")

	// Update delegation state
	if err := s.db.UpdateBTCDelegationState(
		ctx,
		unbondedEarlyEvent.StakingTxHash,
		types.QualifiedStatesForUnbondedEarly(),
		types.StateUnbonding,
		db.WithSubState(subState),
		db.WithBbnHeight(bbnBlockHeight),
		db.WithUnbondingBTCTimestamp(unbondingBtcTimestamp),
		db.WithUnbondingStartHeight(unbondingStartHeight),
		db.WithBbnEventType(types.EventBTCDelgationUnbondedEarly),
	); err != nil {
		if db.IsNotFoundError(err) {
			// maybe the btc notifier has already identified the unbonding tx and updated the state
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Interface("qualified_states", types.QualifiedStatesForUnbondedEarly()).
				Msg("delegation not in qualified states for early unbonding update")
			return nil
		}

		return fmt.Errorf("failed to update BTC delegation state: %w", err)
	}

	return nil
}

func (s *Service) processBTCDelegationExpiredEvent(
	ctx context.Context, event abcitypes.Event, bbnBlockHeight int64,
) error {
	expiredEvent, err := parseEvent[*bbntypes.EventBTCDelegationExpired](
		types.EventBTCDelegationExpired,
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
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", dbErr)
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
		return fmt.Errorf("failed to save timelock expire: %w", err)
	}

	// Update delegation state
	if err := s.db.UpdateBTCDelegationState(
		ctx,
		delegation.StakingTxHashHex,
		types.QualifiedStatesForExpired(),
		types.StateUnbonding,
		db.WithSubState(subState),
		db.WithBbnHeight(bbnBlockHeight),
		db.WithBbnEventType(types.EventBTCDelegationExpired),
	); err != nil {
		return fmt.Errorf("failed to update BTC delegation state: %w", err)
	}

	return nil
}
