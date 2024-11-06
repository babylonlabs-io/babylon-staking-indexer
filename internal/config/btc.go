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
	RPCHost              string        `long:"rpchost" description:"The daemon's rpc listening address."`
	RPCUser              string        `long:"rpcuser" description:"Username for RPC connections."`
	RPCPass              string        `long:"rpcpass" default-mask:"-" description:"Password for RPC connections."`
	PrunedNodeMaxPeers   int           `long:"pruned-node-max-peers" description:"The maximum number of peers staker will choose from the backend node to retrieve pruned blocks from. This only applies to pruned nodes."`
	BlockPollingInterval time.Duration `long:"blockpollinginterval" description:"The interval that will be used to poll bitcoind for new blocks. Only used if rpcpolling is true."`
	TxPollingInterval    time.Duration `long:"txpollinginterval" description:"The interval that will be used to poll bitcoind for new tx. Only used if rpcpolling is true."`
	BlockCacheSize       uint64        `long:"block-cache-size" description:"Size of the Bitcoin blocks cache."`
	MaxRetryTimes        uint          `long:"max-retry-times" description:"The max number of retries to an RPC call in case of failure."`
	RetryInterval        time.Duration `long:"retry-interval" description:"The time interval between each retry."`
	NetParams            string        `long:"net-params" description:"The network parameters to use for the Bitcoin client."`
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
