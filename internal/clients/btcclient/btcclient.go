package btcclient

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
)

type BtcClient struct {
	client *rpcclient.Client

	params *chaincfg.Params
	cfg    *config.BTCConfig
}

func NewBtcClient(cfg *config.BTCConfig) (*BtcClient, error) {
	params, err := utils.GetBTCParams(cfg.NetParams)
	if err != nil {
		return nil, err
	}

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Endpoint,
		HTTPPostMode: true,
		User:         cfg.Username,
		Pass:         cfg.Password,
		DisableTLS:   false,
		Params:       params.Name,
	}

	rpcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	return &BtcClient{
		client: rpcClient,
		params: params,
		cfg:    cfg,
	}, nil
}

func (b *BtcClient) GetBlockCount() (int64, error) {
	return metrics.RecordBtcClientMetrics[int64](b.client.GetBlockCount)
}
