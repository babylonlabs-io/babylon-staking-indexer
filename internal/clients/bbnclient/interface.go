package bbnclient

import (
	"context"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
)

type BbnInterface interface {
	GetLatestBlockNumber(ctx context.Context) (int64, *types.Error)
	GetBlockResults(
		ctx context.Context, blockHeight int64,
	) (*ctypes.ResultBlockResults, *types.Error)
}
