package bbnclient

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	queryClient         *query.QueryClient
	cfg                 *config.BBNConfig
	subscriptionChanMap map[string]chan ctypes.ResultEvent
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
	return &BBNClient{
		queryClient:         queryClient,
		cfg:                 cfg,
		subscriptionChanMap: make(map[string]chan ctypes.ResultEvent),
	}
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

func (c *BBNClient) GetBlock(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlock, error) {
	callForBlock := func() (*ctypes.ResultBlock, error) {
		resp, err := c.queryClient.RPCClient.Block(ctx, blockHeight)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	block, err := clientCallWithRetry(callForBlock, c.cfg)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (c *BBNClient) Subscribe(
	subscriber, query string,
	healthCheckInterval time.Duration,
	maxEventWaitInterval time.Duration,
	outCapacity ...int,
) (out <-chan ctypes.ResultEvent, err error) {
	// Create a new channel for this subscriber if it doesn't exist
	if _, exists := c.subscriptionChanMap[subscriber]; !exists {
		c.subscriptionChanMap[subscriber] = make(chan ctypes.ResultEvent)
	}

	var rawEventChan <-chan ctypes.ResultEvent
	subscribe := func() error {
		rawEventChan, err = c.queryClient.RPCClient.Subscribe(
			context.Background(),
			subscriber,
			query,
			outCapacity...,
		)
		if err != nil {
			return fmt.Errorf("failed to subscribe babylon events for query %s: %w", query, err)
		}
		return nil
	}

	if err := subscribe(); err != nil {
		return nil, err
	}

	go func() {
		timeoutTicker := time.NewTicker(healthCheckInterval)
		defer timeoutTicker.Stop()
		lastEventTime := time.Now()

		for {
			select {
			case event, ok := <-rawEventChan:
				if !ok {
					log.Error().
						Str("subscriber", subscriber).
						Str("query", query).
						Msg("Subscription channel closed")
					return
				}
				lastEventTime = time.Now()
				c.subscriptionChanMap[subscriber] <- event

			case <-timeoutTicker.C:
				if time.Since(lastEventTime) > maxEventWaitInterval {
					log.Error().
						Str("subscriber", subscriber).
						Str("query", query).
						Dur("healthCheckInterval", healthCheckInterval).
						Dur("maxEventWaitInterval", maxEventWaitInterval).
						Msg("No events received, attempting to resubscribe")

					if err := c.queryClient.RPCClient.Unsubscribe(
						context.Background(),
						subscriber,
						query,
					); err != nil {
						log.Error().Err(err).Msg("Failed to unsubscribe babylon events")
					}

					if err := subscribe(); err != nil {
						log.Error().Err(err).Msg("Failed to resubscribe babylon events")
					} else {
						log.Info().
							Str("subscriber", subscriber).
							Str("query", query).
							Msg("Successfully resubscribed babylon events")
						// reset last event time
						lastEventTime = time.Now()
					}
				}
			}
		}
	}()

	return c.subscriptionChanMap[subscriber], nil
}

func (c *BBNClient) UnsubscribeAll(subscriber string) error {
	if ch, exists := c.subscriptionChanMap[subscriber]; exists {
		close(ch)
		delete(c.subscriptionChanMap, subscriber)
	}
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
