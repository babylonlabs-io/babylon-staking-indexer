package db

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
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
	 * SaveGlobalParams saves the global parameters to the database.
	 * If the document already exists by type and version, it will be skipped.
	 * @param ctx The context
	 * @param param The global parameters document
	 * @return An error if the operation failed
	 */
	SaveGlobalParams(
		ctx context.Context, param *model.GolablParamDocument,
	) error
}
