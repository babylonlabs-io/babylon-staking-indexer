package btcclient

import (
	"fmt"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/rs/zerolog"
	"context"
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

type BlockCountResponse struct {
	count int64
}

func (c *BTCClient) GetTipHeight(ctx context.Context) (uint64, error) {
	callForBlockCount := func() (*BlockCountResponse, error) {
		count, err := c.client.GetBlockCount()
		if err != nil {
			return nil, err
		}

		return &BlockCountResponse{count: count}, nil
	}

	blockCount, err := clientCallWithRetry(callForBlockCount, c.cfg, log.Ctx(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to get block count: %w", err)
	}

	return uint64(blockCount.count), nil
}

func clientCallWithRetry[T any](
	call retry.RetryableFuncWithData[*T], cfg *config.BTCConfig, log *zerolog.Logger,
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
