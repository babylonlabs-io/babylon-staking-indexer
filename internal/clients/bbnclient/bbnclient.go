package bbnclient

import (
	"context"
	"net/http"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/client/query"
)

type BbnClient struct {
	queryClient *query.QueryClient
}

func NewBbnClient(cfg *config.BabylonQueryConfig) BbnInterface {
	queryClient, err := query.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating BBN query client")
	}
	return &BbnClient{queryClient}
}

func (c *BbnClient) GetLatestBlockNumber(ctx context.Context) (int64, *types.Error) {
	status, err := c.queryClient.RPCClient.Status(ctx)
	if err != nil {
		return 0, types.NewErrorWithMsg(
			http.StatusInternalServerError, types.InternalServiceError, err.Error(),
		)
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

func (c *BbnClient) GetBlockResults(ctx context.Context, blockHeight int64) (*ctypes.ResultBlockResults, *types.Error) {
	resp, err := c.queryClient.RPCClient.BlockResults(ctx, &blockHeight)
	if err != nil {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError, types.InternalServiceError, err.Error(),
		)
	}

	return resp, nil
}

func (c *BbnClient) getBlockResults(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlockResults, *types.Error) {
	resp, err := c.queryClient.RPCClient.BlockResults(ctx, blockHeight)
	if err != nil {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError, types.InternalServiceError, err.Error(),
		)
	}
	return resp, nil
}
