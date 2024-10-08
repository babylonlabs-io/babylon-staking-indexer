package bbnclient

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient/bbntypes"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
)

type BbnInterface interface {
	GetHealthCheckStatus(ctx context.Context) (bool, *types.Error)
	GetLatestBlockNumber(ctx context.Context) (int, *types.Error)
	GetBlockResults(ctx context.Context, height int) (
		*bbntypes.BlockResultsResponse, *types.Error,
	)
}
