package config

import (
	"errors"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
)

// BTCConfig defines configuration for the Bitcoin client
type BTCConfig struct {
	Endpoint          string `mapstructure:"endpoint"`
	DisableTLS        bool   `mapstructure:"disable-tls"`
	EstimateMode      string `mapstructure:"estimate-mode"` // the BTC tx fee estimate mode, which is only used by bitcoind, must be either ECONOMICAL or CONSERVATIVE
	NetParams         string `mapstructure:"net-params"`
	Username          string `mapstructure:"username"`
	Password          string `mapstructure:"password"`
	ReconnectAttempts int    `mapstructure:"reconnect-attempts"`
	ZmqSeqEndpoint    string `mapstructure:"zmq-seq-endpoint"`
	ZmqBlockEndpoint  string `mapstructure:"zmq-block-endpoint"`
	ZmqTxEndpoint     string `mapstructure:"zmq-tx-endpoint"`
}

func (cfg *BTCConfig) Validate() error {
	if cfg.ReconnectAttempts < 0 {
		return errors.New("reconnect-attempts must be non-negative")
	}

	if _, ok := utils.GetValidNetParams()[cfg.NetParams]; !ok {
		return errors.New("invalid net params")
	}

	// TODO: implement regex validation for zmq endpoint
	if cfg.ZmqBlockEndpoint == "" {
		return errors.New("zmq block endpoint cannot be empty")
	}

	if cfg.ZmqTxEndpoint == "" {
		return errors.New("zmq tx endpoint cannot be empty")
	}

	if cfg.ZmqSeqEndpoint == "" {
		return errors.New("zmq seq endpoint cannot be empty")
	}

	if cfg.EstimateMode != "ECONOMICAL" && cfg.EstimateMode != "CONSERVATIVE" {
		return errors.New("estimate-mode must be either ECONOMICAL or CONSERVATIVE when the backend is bitcoind")
	}

	return nil
}

const (
	// Config for polling jittner in bitcoind client, with polling enabled
	DefaultTxPollingJitter     = 0.5
	DefaultRpcBtcNodeHost      = "127.0.01:18556"
	DefaultBtcNodeRpcUser      = "rpcuser"
	DefaultBtcNodeRpcPass      = "rpcpass"
	DefaultBtcNodeEstimateMode = "CONSERVATIVE"
	DefaultBtcblockCacheSize   = 20 * 1024 * 1024 // 20 MB
	DefaultZmqSeqEndpoint      = "tcp://127.0.0.1:28333"
	DefaultZmqBlockEndpoint    = "tcp://127.0.0.1:29001"
	DefaultZmqTxEndpoint       = "tcp://127.0.0.1:29002"
)

func DefaultBTCConfig() BTCConfig {
	return BTCConfig{
		Endpoint:          DefaultRpcBtcNodeHost,
		DisableTLS:        true,
		EstimateMode:      DefaultBtcNodeEstimateMode,
		NetParams:         utils.BtcSimnet.String(),
		Username:          DefaultBtcNodeRpcUser,
		Password:          DefaultBtcNodeRpcPass,
		ReconnectAttempts: 3,
		ZmqSeqEndpoint:    DefaultZmqSeqEndpoint,
		ZmqBlockEndpoint:  DefaultZmqBlockEndpoint,
		ZmqTxEndpoint:     DefaultZmqTxEndpoint,
	}
}
