package config

import (
	"fmt"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
	"github.com/btcsuite/btcd/rpcclient"
)

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

// BTCConfig defines configuration for the Bitcoin client
type BTCConfig struct {
	RPCHost              string        `mapstructure:"rpchost"`
	RPCUser              string        `mapstructure:"rpcuser"`
	RPCPass              string        `mapstructure:"rpcpass"`
	PrunedNodeMaxPeers   int           `mapstructure:"prunednodemaxpeers"`
	BlockPollingInterval time.Duration `mapstructure:"blockpollinginterval"`
	TxPollingInterval    time.Duration `mapstructure:"txpollinginterval"`
	BlockCacheSize       uint64        `mapstructure:"blockcachesize"`
	MaxRetryTimes        uint          `mapstructure:"maxretrytimes"`
	RetryInterval        time.Duration `mapstructure:"retryinterval"`
	NetParams            string        `mapstructure:"netparams"`
}

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
	}
}

func (cfg *BTCConfig) ToConnConfig() *rpcclient.ConnConfig {
	return &rpcclient.ConnConfig{
		Host:                 cfg.RPCHost,
		User:                 cfg.RPCUser,
		Pass:                 cfg.RPCPass,
		DisableTLS:           true,
		DisableConnectOnNew:  true,
		DisableAutoReconnect: false,
		// we use post mode as it sure it works with either bitcoind or btcwallet
		// we may need to re-consider it later if we need any notifications
		HTTPPostMode: true,
	}
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
