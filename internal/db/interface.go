package db

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
)

type DbInterface interface {
	/**
	 * Ping checks the database connection.
	 * @param ctx The context
	 * @return An error if the operation failed
	 */
	Ping(ctx context.Context) error
	/**
	 * SaveNewFinalityProvider saves a new finality provider to the database.
	 * If the finality provider already exists, DuplicateKeyError will be returned.
	 * @param ctx The context
	 * @param fpDoc The finality provider details
	 * @return An error if the operation failed
	 */
	SaveNewFinalityProvider(
		ctx context.Context, fpDoc *model.FinalityProviderDetails,
	) error
	/**
	 * UpdateFinalityProviderState updates the finality provider state.
	 * @param ctx The context
	 * @param btcPk The BTC public key
	 * @param newState The new state
	 * @return An error if the operation failed
	 */
	UpdateFinalityProviderState(
		ctx context.Context, btcPk string, newState string,
	) error
	/**
	 * UpdateFinalityProviderDetailsFromEvent updates the finality provider details based on the event.
	 * Only the fields that are not empty in the event will be updated.
	 * @param ctx The context
	 * @param detailsToUpdate The finality provider details to update
	 * @return An error if the operation failed
	 */
	UpdateFinalityProviderDetailsFromEvent(
		ctx context.Context, detailsToUpdate *model.FinalityProviderDetails,
	) error
	/**
	 * GetFinalityProviderByBtcPk retrieves the finality provider details by the BTC public key.
	 * If the finality provider does not exist, a NotFoundError will be returned.
	 * @param ctx The context
	 * @param btcPk The BTC public key
	 * @return The finality provider details or an error
	 */
	GetFinalityProviderByBtcPk(
		ctx context.Context, btcPk string,
	) (*model.FinalityProviderDetails, error)
	/**
	 * SaveStakingParams saves the staking parameters to the database.
	 * @param ctx The context
	 * @param version The version of the staking parameters
	 * @param params The staking parameters
	 * @return An error if the operation failed
	 */
	SaveStakingParams(
		ctx context.Context, version uint32, params *bbnclient.StakingParams,
	) error
	/**
	 * GetStakingParams retrieves the staking parameters by the version.
	 * @param ctx The context
	 * @param version The version of the staking parameters
	 * @return The staking parameters or an error
	 */
	GetStakingParams(ctx context.Context, version uint32) (*bbnclient.StakingParams, error)
	/**
	 * SaveCheckpointParams saves the checkpoint parameters to the database.
	 * @param ctx The context
	 * @param params The checkpoint parameters
	 * @return An error if the operation failed
	 */
	SaveCheckpointParams(
		ctx context.Context, params *bbnclient.CheckpointParams,
	) error
	/**
	 * SaveNewBTCDelegation saves a new BTC delegation to the database.
	 * If the BTC delegation already exists, DuplicateKeyError will be returned.
	 * @param ctx The context
	 * @param delegationDoc The BTC delegation details
	 * @return An error if the operation failed
	 */
	SaveNewBTCDelegation(
		ctx context.Context, delegationDoc *model.BTCDelegationDetails,
	) error
	/**
	 * SaveBTCDelegationStateUpdate saves a BTC delegation state update to the database.
	 * @param ctx The context
	 * @param delegationDoc The BTC delegation details
	 * @return An error if the operation failed
	 */
	UpdateBTCDelegationState(
		ctx context.Context,
		stakingTxHash string,
		newState types.DelegationState,
		subState *types.DelegationSubState,
	) error
	/**
	 * SaveBTCDelegationUnbondingCovenantSignature saves a BTC delegation
	 * unbonding covenant signature to the database.
	 * @param ctx The context
	 * @param stakingTxHash The staking tx hash
	 * @param covenantBtcPkHex The covenant BTC public key
	 * @param signatureHex The signature
	 * @return An error if the operation failed
	 */
	SaveBTCDelegationUnbondingCovenantSignature(
		ctx context.Context, stakingTxHash string, covenantBtcPkHex string, signatureHex string,
	) error
	/**
	 * GetBTCDelegationState retrieves the BTC delegation state.
	 * @param ctx The context
	 * @param stakingTxHash The staking tx hash
	 * @return The BTC delegation state or an error
	 */
	GetBTCDelegationState(ctx context.Context, stakingTxHash string) (*types.DelegationState, error)
	/**
	 * UpdateBTCDelegationDetails updates the BTC delegation details.
	 * @param ctx The context
	 * @param stakingTxHash The staking tx hash
	 * @param details The BTC delegation details to update
	 * @return An error if the operation failed
	 */
	UpdateBTCDelegationDetails(
		ctx context.Context, stakingTxHash string, details *model.BTCDelegationDetails,
	) error
	/**
	 * GetBTCDelegationByStakingTxHash retrieves the BTC delegation details by the staking tx hash.
	 * If the BTC delegation does not exist, a NotFoundError will be returned.
	 * @param ctx The context
	 * @param stakingTxHash The staking tx hash
	 * @return The BTC delegation details or an error
	 */
	GetBTCDelegationByStakingTxHash(
		ctx context.Context, stakingTxHash string,
	) (*model.BTCDelegationDetails, error)
	/**
	 * UpdateDelegationsStateByFinalityProvider updates the BTC delegation state by the finality provider public key.
	 * @param ctx The context
	 * @param fpBtcPkHex The finality provider public key
	 * @param newState The new state
	 * @param qualifiedStates The qualified states
	 * @return An error if the operation failed
	 */
	UpdateDelegationsStateByFinalityProvider(
		ctx context.Context, fpBtcPkHex string, newState types.DelegationState,
	) error
	/**
	 * SaveNewTimeLockExpire saves a new timelock expire to the database.
	 * If the timelock expire already exists, DuplicateKeyError will be returned.
	 * @param ctx The context
	 * @param stakingTxHashHex The staking tx hash hex
	 * @param expireHeight The expire height
	 * @param txType The transaction type
	 * @return An error if the operation failed
	 */
	SaveNewTimeLockExpire(
		ctx context.Context,
		stakingTxHashHex string,
		expireHeight uint32,
		subState types.DelegationSubState,
	) error
	/**
	 * FindExpiredDelegations finds the expired delegations.
	 * @param ctx The context
	 * @param btcTipHeight The BTC tip height
	 * @return The expired delegations or an error
	 */
	FindExpiredDelegations(ctx context.Context, btcTipHeight, limit uint64) ([]model.TimeLockDocument, error)
	/**
	 * DeleteExpiredDelegation deletes an expired delegation.
	 * @param ctx The context
	 * @param id The ID of the expired delegation
	 * @return An error if the operation failed
	 */
	DeleteExpiredDelegation(ctx context.Context, stakingTxHashHex string) error
	/**
	 * GetLastProcessedBbnHeight retrieves the last processed BBN height.
	 * @param ctx The context
	 * @return The last processed height or an error
	 */
	GetLastProcessedBbnHeight(ctx context.Context) (uint64, error)
	/**
	 * UpdateLastProcessedBbnHeight updates the last processed BBN height.
	 * @param ctx The context
	 * @param height The last processed height
	 * @return An error if the operation failed
	 */
	UpdateLastProcessedBbnHeight(ctx context.Context, height uint64) error
}
