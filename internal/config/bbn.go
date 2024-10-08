package config

import (
	"fmt"
)

type BbnConfig struct {
	// Endpoint specifies the URL of the BBN RPC server without the protocol prefix (http:// or https://).
	Endpoint string `mapstructure:"endpoint"`
	Port     string `mapstructure:"port"`
	Timeout  int    `mapstructure:"timeout"`
}

func (cfg *BbnConfig) Validate() error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("Babylon node endpoint is required")
	}
	if cfg.Port == "" {
		return fmt.Errorf("Babylon node port is required")
	}
	if cfg.Timeout == 0 {
		return fmt.Errorf("Babylon node timeout is required")
	}

	return nil
}
