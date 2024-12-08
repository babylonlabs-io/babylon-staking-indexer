package config

import (
	"fmt"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	"github.com/btcsuite/btcd/rpcclient"
)

// BTCConfig defines configuration for the Bitcoin client
type BTCConfig struct {
	RPCHost                 string        `mapstructure:"rpchost"`
	RPCUser                 string        `mapstructure:"rpcuser"`
	RPCPass                 string        `mapstructure:"rpcpass"`
	PrunedNodeMaxPeers      int           `mapstructure:"prunednodemaxpeers"`
	BlockPollingInterval    time.Duration `mapstructure:"blockpollinginterval"`
	TxPollingInterval       time.Duration `mapstructure:"txpollinginterval"`
	TxPollingIntervalJitter float64       `mapstructure:"txpollingintervaljitter"`
	BlockCacheSize          uint64        `mapstructure:"blockcachesize"`
	MaxRetryTimes           uint          `mapstructure:"maxretrytimes"`
	RetryInterval           time.Duration `mapstructure:"retryinterval"`
	NetParams               string        `mapstructure:"netparams"`
}

func (cfg *BTCConfig) ToConnConfig() (*rpcclient.ConnConfig, error) {
	params, err := utils.GetBTCParams(cfg.NetParams)
	if err != nil {
		return nil, fmt.Errorf("invalid BTC network params: %w", err)
	}

	return &rpcclient.ConnConfig{
		Host:                 cfg.RPCHost,
		User:                 cfg.RPCUser,
		Pass:                 cfg.RPCPass,
		DisableTLS:           true,
		Params:               params.Name,
		DisableConnectOnNew:  true,
		DisableAutoReconnect: false,
		// we use post mode as it sure it works with either bitcoind or btcwallet
		// we may need to re-consider it later if we need any notifications
		HTTPPostMode: true,
	}, nil
}

func (cfg *BTCConfig) Validate() error {
	if cfg.RPCHost == "" {
		return fmt.Errorf("RPC host cannot be empty")
	}
	if cfg.RPCUser == "" {
		return fmt.Errorf("RPC user cannot be empty")
	}
	if cfg.RPCPass == "" {
		return fmt.Errorf("RPC password cannot be empty")
	}

	if cfg.BlockPollingInterval <= 0 {
		return fmt.Errorf("block polling interval should be positive")
	}
	if cfg.TxPollingInterval <= 0 {
		return fmt.Errorf("tx polling interval should be positive")
	}
	if cfg.TxPollingIntervalJitter < 0 || cfg.TxPollingIntervalJitter > 1 {
		return fmt.Errorf("tx polling interval jitter should be between 0 and 1")
	}

	if cfg.BlockCacheSize <= 0 {
		return fmt.Errorf("block cache size should be positive")
	}

	if cfg.MaxRetryTimes <= 0 {
		return fmt.Errorf("max retry times should be positive")
	}

	if cfg.RetryInterval <= 0 {
		return fmt.Errorf("retry interval should be positive")
	}

	if _, ok := utils.GetValidNetParams()[cfg.NetParams]; !ok {
		return fmt.Errorf("invalid net params")
	}

	return nil
}

const (
	// default rpc port of signet is 38332
	defaultBitcoindRpcHost        = "127.0.0.1:38332"
	defaultBitcoindRPCUser        = "user"
	defaultBitcoindRPCPass        = "pass"
	defaultBitcoindBlockCacheSize = 20 * 1024 * 1024 // 20 MB
	defaultBlockPollingInterval   = 30 * time.Second
	defaultTxPollingInterval      = 30 * time.Second
	defaultMaxRetryTimes          = 5
	defaultRetryInterval          = 500 * time.Millisecond
	// DefaultTxPollingJitter defines the default TxPollingIntervalJitter
	// to be used for bitcoind backend.
	DefaultTxPollingJitter = 0.5
)

func DefaultBTCConfig() *BTCConfig {
	return &BTCConfig{
		RPCHost:              defaultBitcoindRpcHost,
		RPCUser:              defaultBitcoindRPCUser,
		RPCPass:              defaultBitcoindRPCPass,
		BlockPollingInterval: defaultBlockPollingInterval,
		TxPollingInterval:    defaultTxPollingInterval,
		BlockCacheSize:       defaultBitcoindBlockCacheSize,
		MaxRetryTimes:        defaultMaxRetryTimes,
		RetryInterval:        defaultRetryInterval,
		NetParams:            "regtest",
	}
}
