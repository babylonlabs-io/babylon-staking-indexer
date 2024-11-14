package bbnclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	bbncfg "github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/client/query"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btcstakingtypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/rs/zerolog/log"
)

type BBNClient struct {
	queryClient *query.QueryClient
	cfg         *config.BBNConfig
}

func NewBBNClient(cfg *config.BBNConfig) BbnInterface {
	bbnQueryCfg := &bbncfg.BabylonQueryConfig{
		RPCAddr: cfg.RPCAddr,
		Timeout: cfg.Timeout,
	}

	queryClient, err := query.New(bbnQueryCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating BBN query client")
	}
	return &BBNClient{queryClient, cfg}
}

func (c *BBNClient) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	callForStatus := func() (*ctypes.ResultStatus, error) {
		status, err := c.queryClient.RPCClient.Status(ctx)
		if err != nil {
			return nil, err
		}
		return status, nil
	}

	status, err := clientCallWithRetry(callForStatus, c.cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block number by fetching status: %w", err)
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

func (c *BBNClient) GetCheckpointParams(ctx context.Context) (*CheckpointParams, error) {
	callForCheckpointParams := func() (*btcctypes.QueryParamsResponse, error) {
		params, err := c.queryClient.BTCCheckpointParams()
		if err != nil {
			return nil, err
		}
		return params, nil
	}

	params, err := clientCallWithRetry(callForCheckpointParams, c.cfg)
	if err != nil {
		return nil, err
	}
	if err := params.Params.Validate(); err != nil {
		return nil, err
	}
	return FromBbnCheckpointParams(params.Params), nil
}

func (c *BBNClient) GetAllStakingParams(ctx context.Context) (map[uint32]*StakingParams, error) {
	allParams := make(map[uint32]*StakingParams)
	version := uint32(0)

	for {
		// First try without retry to check for ErrParamsNotFound
		params, err := c.queryClient.BTCStakingParamsByVersion(version)
		if err != nil {
			if strings.Contains(err.Error(), btcstakingtypes.ErrParamsNotFound.Error()) {
				break // Exit loop if params not found
			}

			// Only retry for other errors
			callForStakingParams := func() (*btcstakingtypes.QueryParamsByVersionResponse, error) {
				return c.queryClient.BTCStakingParamsByVersion(version)
			}

			params, err = clientCallWithRetry(callForStakingParams, c.cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to get staking params for version %d: %w", version, err)
			}
		}

		if err := params.Params.Validate(); err != nil {
			return nil, fmt.Errorf("failed to validate staking params for version %d: %w", version, err)
		}

		allParams[version] = FromBbnStakingParams(params.Params)
		version++
	}

	if len(allParams) == 0 {
		return nil, fmt.Errorf("no staking params found")
	}

	return allParams, nil
}

func (c *BBNClient) GetBlockResults(
	ctx context.Context, blockHeight *int64,
) (*ctypes.ResultBlockResults, error) {
	callForBlockResults := func() (*ctypes.ResultBlockResults, error) {
		resp, err := c.queryClient.RPCClient.BlockResults(ctx, blockHeight)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	blockResults, err := clientCallWithRetry(callForBlockResults, c.cfg)
	if err != nil {
		return nil, err
	}
	return blockResults, nil
}

func (c *BBNClient) Subscribe(subscriber, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error) {
	return c.queryClient.RPCClient.Subscribe(context.Background(), subscriber, query, outCapacity...)
}

func (c *BBNClient) UnsubscribeAll(subscriber string) error {
	return c.queryClient.RPCClient.UnsubscribeAll(context.Background(), subscriber)
}

func (c *BBNClient) IsRunning() bool {
	return c.queryClient.RPCClient.IsRunning()
}

func (c *BBNClient) Start() error {
	return c.queryClient.RPCClient.Start()
}

func clientCallWithRetry[T any](
	call retry.RetryableFuncWithData[*T], cfg *config.BBNConfig,
) (*T, error) {
	result, err := retry.DoWithData(call, retry.Attempts(cfg.MaxRetryTimes), retry.Delay(cfg.RetryInterval), retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			log.Debug().
				Uint("attempt", n+1).
				Uint("max_attempts", cfg.MaxRetryTimes).
				Err(err).
				Msg("failed to call the RPC client")
		}))

	if err != nil {
		return nil, err
	}
	return result, nil
}
