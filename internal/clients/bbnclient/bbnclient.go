package bbnclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	bbncfg "github.com/babylonlabs-io/babylon/v3/client/config"
	"github.com/babylonlabs-io/babylon/v3/client/query"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btcstakingtypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/rs/zerolog/log"
)

type BBNClient struct {
	queryClient *query.QueryClient
	cfg         *config.BBNConfig
}

func NewBBNClient(cfg *config.BBNConfig) (BbnInterface, error) {
	bbnQueryCfg := &bbncfg.BabylonQueryConfig{
		RPCAddr: cfg.RPCAddr,
		Timeout: cfg.Timeout,
	}

	queryClient, err := query.New(bbnQueryCfg)
	if err != nil {
		return nil, err
	}
	return &BBNClient{
		queryClient: queryClient,
		cfg:         cfg,
	}, nil
}

func (c *BBNClient) GetChainID(ctx context.Context) (string, error) {
	status, err := c.getStatus(ctx)
	if err != nil {
		return "", err
	}

	return status.NodeInfo.Network, nil
}

func (c *BBNClient) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	status, err := c.getStatus(ctx)
	if err != nil {
		return 0, err
	}

	return status.SyncInfo.LatestBlockHeight, nil
}

func (c *BBNClient) getStatus(ctx context.Context) (*ctypes.ResultStatus, error) {
	callForStatus := func() (*ctypes.ResultStatus, error) {
		status, err := c.queryClient.RPCClient.Status(ctx)
		if err != nil {
			return nil, err
		}
		return status, nil
	}

	status, err := clientCallWithRetry(ctx, callForStatus, c.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	return status, nil
}

func (c *BBNClient) GetCheckpointParams(ctx context.Context) (*CheckpointParams, error) {
	callForCheckpointParams := func() (*btcctypes.QueryParamsResponse, error) {
		params, err := c.queryClient.BTCCheckpointParams()
		if err != nil {
			return nil, err
		}
		return params, nil
	}

	params, err := clientCallWithRetry(ctx, callForCheckpointParams, c.cfg)
	if err != nil {
		return nil, err
	}
	if err := params.Params.Validate(); err != nil {
		return nil, err
	}
	return FromBbnCheckpointParams(params.Params), nil
}

func (c *BBNClient) GetAllStakingParams(ctx context.Context) (map[uint32]*StakingParams, error) {
	return c.GetStakingParams(ctx, 0)
}

func (c *BBNClient) GetStakingParams(ctx context.Context, minVersion uint32) (map[uint32]*StakingParams, error) {
	allParams := make(map[uint32]*StakingParams)

	for version := minVersion; ; version++ {
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

			params, err = clientCallWithRetry(ctx, callForStakingParams, c.cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to get staking params for version %d: %w", version, err)
			}
		}

		/*
			TODO: uncomment these lines once all migrations are done by devops
			The reason why we skip validation is because the new version of bbn client return error
			if staking params.MaxFinalityProviders == 0, which is the case for now

			if err := params.Params.Validate(); err != nil {
				return nil, fmt.Errorf("failed to validate staking params for version %d: %w", version, err)
			}*/

		allParams[version] = FromBbnStakingParams(params.Params)
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

	blockResults, err := clientCallWithRetry(ctx, callForBlockResults, c.cfg)
	if err != nil {
		return nil, err
	}
	return blockResults, nil
}

func (c *BBNClient) BabylonStakerAddress(ctx context.Context, stakingTxHashHex string) (string, error) {
	call := func() (*string, error) {
		resp, err := c.queryClient.BTCDelegation(stakingTxHashHex)
		if err != nil {
			return nil, err
		}

		return &resp.BtcDelegation.StakerAddr, nil
	}

	stakerAddr, err := clientCallWithRetry(ctx, call, c.cfg)
	if err != nil {
		return "", err
	}

	return *stakerAddr, nil
}

func (c *BBNClient) GetBlock(ctx context.Context, blockHeight *int64) (*ctypes.ResultBlock, error) {
	callForBlock := func() (*ctypes.ResultBlock, error) {
		resp, err := c.queryClient.RPCClient.Block(ctx, blockHeight)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	block, err := clientCallWithRetry(ctx, callForBlock, c.cfg)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (c *BBNClient) Subscribe(
	ctx context.Context,
	subscriber, query string,
	healthCheckInterval time.Duration,
	maxEventWaitInterval time.Duration,
	outCapacity ...int,
) (out <-chan ctypes.ResultEvent, err error) {
	eventChan := make(chan ctypes.ResultEvent)

	subscribe := func() (<-chan ctypes.ResultEvent, error) {
		newChan, err := c.queryClient.RPCClient.Subscribe(
			context.Background(),
			subscriber,
			query,
			outCapacity...,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to subscribe babylon events for query %s: %w", query, err,
			)
		}
		return newChan, nil
	}

	// Initial subscription
	rawEventChan, err := subscribe()
	if err != nil {
		close(eventChan)
		return nil, err
	}
	go func() {
		defer close(eventChan)
		timeoutTicker := time.NewTicker(healthCheckInterval)
		defer timeoutTicker.Stop()
		lastEventTime := time.Now()

		log := log.Ctx(ctx)
		for {
			select {
			case event, ok := <-rawEventChan:
				if !ok {
					log.Fatal().
						Str("subscriber", subscriber).
						Str("query", query).
						Msg("Subscription channel closed, this shall not happen")
				}
				lastEventTime = time.Now()
				eventChan <- event
			case <-timeoutTicker.C:
				if time.Since(lastEventTime) > maxEventWaitInterval {
					log.Error().
						Str("subscriber", subscriber).
						Str("query", query).
						Msg("No events received, attempting to resubscribe")

					if err := c.queryClient.RPCClient.Unsubscribe(
						context.Background(),
						subscriber,
						query,
					); err != nil {
						log.Error().Err(err).Msg("Failed to unsubscribe babylon events")
					}

					// Create new subscription
					newEventChan, err := subscribe()
					if err != nil {
						log.Error().Err(err).Msg("Failed to resubscribe babylon events")
						continue
					}

					// Replace the old channel with the new one
					rawEventChan = newEventChan
					// reset last event time
					lastEventTime = time.Now()
				}
			}
		}
	}()

	return eventChan, nil
}

func (c *BBNClient) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return c.queryClient.RPCClient.UnsubscribeAll(ctx, subscriber)
}

func (c *BBNClient) IsRunning() bool {
	return c.queryClient.RPCClient.IsRunning()
}

func (c *BBNClient) Start() error {
	return c.queryClient.RPCClient.Start()
}

func clientCallWithRetry[T any](
	ctx context.Context, call retry.RetryableFuncWithData[*T], cfg *config.BBNConfig,
) (*T, error) {
	result, err := retry.DoWithData(call, retry.Context(ctx), retry.Attempts(cfg.MaxRetryTimes), retry.Delay(cfg.RetryInterval), retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			log.Ctx(ctx).Debug().
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

// GetWasmAllowlist queries a CosmWasm contract smart state for the current allowlist.
// LCD endpoint: /cosmwasm/wasm/v1/contract/<address>/smart/<base64-query>
// with query message: {"allowed_finality_providers":{}}
func (c *BBNClient) GetWasmAllowlist(ctx context.Context, contractAddress string) ([]string, error) {
	type smartQueryResponse struct {
		Data []string `json:"data"`
	}

	rawQuery := []byte("{\"allowed_finality_providers\":{}}")
	b64 := base64.StdEncoding.EncodeToString(rawQuery)

	// Check if LCD address is configured
	if c.cfg.LCDAddr == "" {
		return nil, fmt.Errorf("LCD address not configured. Please set lcd-addr in configuration to use CosmWasm queries")
	}

	// LCD REST API endpoint for CosmWasm smart contract queries
	url := fmt.Sprintf("%s/cosmwasm/wasm/v1/contract/%s/smart/%s", c.cfg.LCDAddr, contractAddress, b64)

	call := func() (*smartQueryResponse, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		client := &http.Client{Timeout: c.cfg.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			type errorResponse struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			var errResp errorResponse
			if err := json.Unmarshal(body, &errResp); err != nil {
				return nil, fmt.Errorf("LCD query failed with status %d and failed to parse error response: %w", resp.StatusCode, err)
			}
			return nil, fmt.Errorf("LCD query failed (%d): %s", errResp.Code, errResp.Message)
		}

		var out smartQueryResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("failed to unmarshal smart query response: %w", err)
		}
		return &out, nil
	}

	resp, err := clientCallWithRetry(ctx, call, c.cfg)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}
