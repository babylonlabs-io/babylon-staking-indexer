package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/rs/zerolog/log"
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

	if err := s.validateBTCDelegationCreatedEvent(newDelegation); err != nil {
		return err
	}

	delegationDoc, err := model.FromEventBTCDelegationCreated(newDelegation)
	if err != nil {
		return err
	}

	err = s.emitConsumerEvent(ctx, types.StatePending, delegationDoc)
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
		EventBTCDelgationUnbondedEarly, event,
	)
	if err != nil {
		return err
	}

	proceed, err := s.validateBTCDelegationUnbondedEarlyEvent(ctx, unbondedEarlyEvent)
	if err != nil {
		return err
	}
	if !proceed {
		// Ignore the event silently
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

	err = s.emitConsumerEvent(ctx, types.StateUnbonding, delegation)
	if err != nil {
		return err
	}

	unbondingStartHeightInt, parseErr := strconv.ParseUint(unbondedEarlyEvent.StartHeight, 10, 32)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse start height: %w", parseErr),
		)
	}

	unbondingExpireHeight := uint32(unbondingStartHeightInt) + delegation.UnbondingTime
	if dbErr = s.db.SaveNewTimeLockExpire(
		ctx, delegation.StakingTxHashHex, unbondingExpireHeight, types.EarlyUnbondingTxType.String(),
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save timelock expire: %w", dbErr),
		)
	}

	if dbErr = s.db.UpdateBTCDelegationState(
		ctx, unbondedEarlyEvent.StakingTxHash, types.StateUnbonding,
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", dbErr),
		)
	}

	pkScriptBytes, parseErr := hex.DecodeString(delegation.StakingOutputPkScript)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to decode staking tx pk script: %w", parseErr),
		)
	}

	stakingTxHash, parseErr := chainhash.NewHashFromStr(delegation.StakingTxHashHex)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking tx hash: %w", parseErr),
		)
	}

	stakingOutpoint := wire.OutPoint{
		Hash:  *stakingTxHash,
		Index: 0, // unbonding tx has only 1 output
	}

	spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
		&stakingOutpoint,
		pkScriptBytes,
		delegation.StartHeight,
	)
	if btcErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to register spend ntfn for staking tx %s: %w", delegation.StakingTxHashHex, btcErr),
		)
	}

	params, dbErr := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get staking params: %w", dbErr),
		)
	}

	s.wg.Add(1)
	go s.watchForSpendUnbondingTx(spendEv, delegation, params)

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

	proceed, err := s.validateBTCDelegationExpiredEvent(ctx, expiredEvent)
	if err != nil {
		return err
	}
	if !proceed {
		// Ignore the event silently
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

	err = s.emitConsumerEvent(ctx, types.StateUnbonding, delegation)
	if err != nil {
		return err
	}

	if dbErr = s.db.SaveNewTimeLockExpire(
		ctx, delegation.StakingTxHashHex, delegation.EndHeight, types.ExpiredTxType.String(),
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to save timelock expire: %w", dbErr),
		)
	}

	if dbErr = s.db.UpdateBTCDelegationState(
		ctx, expiredEvent.StakingTxHash, types.StateUnbonding,
	); dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to update BTC delegation state: %w", dbErr),
		)
	}

	stakingTxHash, parseErr := chainhash.NewHashFromStr(delegation.StakingTxHashHex)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse staking tx hash: %w", parseErr),
		)
	}

	pkScriptBytes, parseErr := hex.DecodeString(delegation.StakingOutputPkScript)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to decode staking tx pk script: %w", parseErr),
		)
	}

	stakingOutpoint := wire.OutPoint{
		Hash:  *stakingTxHash,
		Index: delegation.StakingOutputIdx,
	}

	spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
		&stakingOutpoint,
		pkScriptBytes,
		delegation.StartHeight,
	)
	if btcErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to register spend ntfn for staking tx %s: %w", delegation.StakingTxHashHex, btcErr),
		)
	}

	params, dbErr := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if dbErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to get staking params: %w", dbErr),
		)
	}

	s.wg.Add(1)
	go s.watchForSpendStakingTx(spendEv, delegation, params)

	return nil
}

func (s *Service) validateBTCDelegationCreatedEvent(event *bbntypes.EventBTCDelegationCreated) *types.Error {
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

func (s *Service) validateCovenantQuorumReachedEvent(ctx context.Context, event *bbntypes.EventCovenantQuorumReached) (bool, *types.Error) {
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

	if event.NewState == bbntypes.BTCDelegationStatus_VERIFIED.String() {
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
	} else if event.NewState == bbntypes.BTCDelegationStatus_ACTIVE.String() {
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

func (s *Service) validateBTCDelegationInclusionProofReceivedEvent(ctx context.Context, event *bbntypes.EventBTCDelegationInclusionProofReceived) (bool, *types.Error) {
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

func (s *Service) validateBTCDelegationUnbondedEarlyEvent(ctx context.Context, event *bbntypes.EventBTCDelgationUnbondedEarly) (bool, *types.Error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"unbonded early event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bbntypes.BTCDelegationStatus_UNBONDED.String() {
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

func (s *Service) validateBTCDelegationExpiredEvent(ctx context.Context, event *bbntypes.EventBTCDelegationExpired) (bool, *types.Error) {
	// Check if the staking tx hash is present
	if event.StakingTxHash == "" {
		return false, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			"expired event missing staking tx hash",
		)
	}

	// Validate the event state
	if event.NewState != bbntypes.BTCDelegationStatus_UNBONDED.String() {
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
