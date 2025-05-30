package btcclient

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"

	"github.com/avast/retry-go/v4"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/rs/zerolog/log"
)

type BTCClient struct {
	client *rpcclient.Client
	cfg    *config.BTCConfig
}

func NewBTCClient(cfg *config.BTCConfig) (*BTCClient, error) {
	connCfg, err := cfg.ToConnConfig()
	if err != nil {
		return nil, err
	}

	c, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	return &BTCClient{
		client: c,
		cfg:    cfg,
	}, nil
}

func (c *BTCClient) GetTipHeight(ctx context.Context) (uint64, error) {
	type BlockCountResponse struct {
		count int64
	}

	callForBlockCount := func() (*BlockCountResponse, error) {
		count, err := c.client.GetBlockCount()
		if err != nil {
			return nil, err
		}

		return &BlockCountResponse{count: count}, nil
	}

	blockCount, err := clientCallWithRetry(ctx, callForBlockCount, c.cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to get block count: %w", err)
	}

	return uint64(blockCount.count), nil
}

func (c *BTCClient) GetBlockTimestamp(ctx context.Context, height uint32) (int64, error) {
	type BlockTimestampResponse struct {
		timestamp int64
	}

	callForBlockTimestamp := func() (*BlockTimestampResponse, error) {
		hash, err := c.client.GetBlockHash(int64(height))
		if err != nil {
			return nil, fmt.Errorf("failed to get block hash at height %d: %w", height, err)
		}

		block, err := c.client.GetBlock(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get block at height %d: %w", height, err)
		}

		return &BlockTimestampResponse{
			timestamp: block.Header.Timestamp.Unix(),
		}, nil
	}

	response, err := clientCallWithRetry(ctx, callForBlockTimestamp, c.cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to get block timestamp: %w", err)
	}

	return response.timestamp, nil
}

func clientCallWithRetry[T any](
	ctx context.Context, call retry.RetryableFuncWithData[*T], cfg *config.BTCConfig,
) (*T, error) {
	result, err := retry.DoWithData(call, retry.Attempts(cfg.MaxRetryTimes), retry.Delay(cfg.RetryInterval), retry.LastErrorOnly(true),
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
