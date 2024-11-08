package bbnclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/client/query"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/rs/zerolog/log"
)

const (
	// Backoff parameters for retries for getting BBN block result
	initialBackoff = 500 * time.Millisecond // Start with 500ms
	backoffFactor  = 2                      // Exponential backoff factor
	maxRetries     = 10                     // 8 minutes in worst case
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

func (c *BbnClient) GetCheckpointParams(ctx context.Context) (*CheckpointParams, *types.Error) {
	params, err := c.queryClient.BTCCheckpointParams()
	if err != nil {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.ClientRequestError,
			fmt.Sprintf("failed to get checkpoint params: %s", err.Error()),
		)
	}
	if err := params.Params.Validate(); err != nil {
		return nil, types.NewErrorWithMsg(
			http.StatusInternalServerError,
			types.ValidationError,
			fmt.Sprintf("failed to validate checkpoint params: %s", err.Error()),
		)
	}
	return FromBbnCheckpointParams(params.Params), nil
}

func (c *BbnClient) GetAllStakingParams(ctx context.Context) (map[uint32]*StakingParams, *types.Error) {
	allParams := make(map[uint32]*StakingParams) // Map to store versioned staking parameters
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
				types.ClientRequestError,
				fmt.Sprintf("failed to get staking params for version %d: %s", version, err.Error()),
			)
		}
		if err := params.Params.Validate(); err != nil {
			return nil, types.NewErrorWithMsg(
				http.StatusInternalServerError,
				types.ValidationError,
				fmt.Sprintf("failed to validate staking params for version %d: %s", version, err.Error()),
			)
		}
		allParams[version] = FromBbnStakingParams(params.Params)
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

// GetBlockResultsWithRetry retries the `getBlockResults` method with exponential backoff
// when the block is not yet available.
func (c *BbnClient) GetBlockResultsWithRetry(
	ctx context.Context, blockHeight *int64,
) (*ctypes.ResultBlockResults, *types.Error) {
	backoff := initialBackoff
	var resp *ctypes.ResultBlockResults
	var err *types.Error

	for i := 0; i < maxRetries; i++ {
		resp, err = c.getBlockResults(ctx, blockHeight)
		if err == nil {
			return resp, nil
		}

		if strings.Contains(
			err.Err.Error(),
			"must be less than or equal to the current blockchain height",
		) {
			log.Debug().
				Str("block_height", fmt.Sprintf("%d", *blockHeight)).
				Str("backoff", backoff.String()).
				Msg("Block not yet available, retrying...")
			time.Sleep(backoff)
			backoff *= backoffFactor
			continue
		}
		return nil, err
	}

	// If we exhaust retries, return a not found error
	return nil, types.NewErrorWithMsg(
		http.StatusNotFound,
		types.NotFound,
		fmt.Sprintf("Block height %d not found after retries", *blockHeight),
	)
}

func (c *BbnClient) Subscribe(subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	return c.queryClient.RPCClient.Subscribe(context.Background(), subscriber, query, outCapacity...)
}

func (c *BbnClient) UnsubscribeAll(subscriber string) error {
	return c.queryClient.RPCClient.UnsubscribeAll(context.Background(), subscriber)
}

func (c *BbnClient) IsRunning() bool {
	return c.queryClient.RPCClient.IsRunning()
}

func (c *BbnClient) Start() error {
	return c.queryClient.RPCClient.Start()
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
