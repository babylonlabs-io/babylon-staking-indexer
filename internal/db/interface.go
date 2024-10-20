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
		ctx context.Context, stakingTxHash string, newState types.DelegationState,
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
}
