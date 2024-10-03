package config

import (
	"fmt"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
)

type BtcConfig struct {
	// Endpoint specifies the URL of the Bitcoin RPC server without the protocol prefix (http:// or https://).
	Endpoint string `mapstructure:"endpoint"`
	/*
		DisableTLS controls the request protocol used for communication.
		When true, connections use HTTP. When false, HTTPS is used for secure communication.
	*/
	DisableTLS bool `mapstructure:"disable-tls"`
	// NetParams defines the network parameters (e.g., mainnet, testnet & signet).
	NetParams string `mapstructure:"net-params"`
	// RpcUser is the username for RPC server authentication.
	RpcUser string `mapstructure:"rpc-user"`
	// RpcPass is the password for RPC server authentication.
	RpcPass string `mapstructure:"rpc-pass"`
}

func (cfg *BtcConfig) Validate() error {
	if _, ok := utils.GetValidNetParams()[cfg.NetParams]; !ok {
		return fmt.Errorf("invalid net params: %v", cfg.NetParams)
	}

	return nil
}
