package db

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
)

type DbInterface interface {
	Ping(ctx context.Context) error
	SaveNewFinalityProvider(
		ctx context.Context, fpDoc *model.FinalityProviderDetails,
	) error
	UpdateFinalityProviderState(
		ctx context.Context, btcPk string, newState string,
	) error
	UpdateFinalityProviderDetailsFromEvent(
		ctx context.Context, detailsToUpdate *model.FinalityProviderDetails,
	) error
	GetFinalityProviderByBtcPk(
		ctx context.Context, btcPk string,
	) (model.FinalityProviderDetails, error)
}
