package bbnclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/client/query"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/rs/zerolog/log"
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
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Sprintf("failed to get latest block number by fetching status: %s", err.Error()),
		)
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

func (c *BbnClient) GetBlockResults(ctx context.Context, blockHeight int64) (*ctypes.ResultBlockResults, *types.Error) {
	resp, err := c.queryClient.RPCClient.BlockResults(ctx, &blockHeight)
	if err != nil {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Sprintf("failed to get block results for block %d: %s", blockHeight, err.Error()),
		)
	}
	return resp, nil
}

func (c *BbnClient) GetCheckpointParams(ctx context.Context) (*CheckpointParams, *types.Error) {
	params, err := c.queryClient.BTCCheckpointParams()
	if err != nil {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Sprintf("failed to get checkpoint params: %s", err.Error()),
		)
	}
	return &params.Params, nil
}

func (c *BbnClient) GetAllStakingParams(ctx context.Context) (map[uint32]StakingParams, *types.Error) {
	allParams := make(map[uint32]StakingParams) // Map to store versioned staking parameters
	version := uint32(0)

	for {
		params, err := c.queryClient.BTCStakingParamsByVersion(version)
		if err != nil {
			if strings.Contains(err.Error(), bbntypes.ErrParamsNotFound.Error()) {
				// Break the loop if an error occurs (indicating no more versions)
				break
			}
			return nil, types.NewErrorWithMsg(
				http.StatusInternalServerError,
				types.InternalServiceError,
				fmt.Sprintf("failed to get staking params for version %d: %s", version, err.Error()),
			)
		}
		allParams[version] = *FromBbnStakingParams(params.Params)
		version++
	}
	if len(allParams) == 0 {
		return nil, types.NewErrorWithMsg(
			http.StatusNotFound,
			types.NotFound,
			"no staking params found",
		)
	}

	return allParams, nil
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
