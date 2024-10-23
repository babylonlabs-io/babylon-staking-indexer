package config

import (
	"errors"
	"time"
)

type PollerConfig struct {
	ParamPollingInterval         time.Duration `mapstructure:"param-polling-interval"`
	ExpiryCheckerPollingInterval time.Duration `mapstructure:"expiry-checker-polling-interval"`
	ExpiredDelegationsLimit      uint64        `mapstructure:"expired-delegations-limit"`
}

func (cfg *PollerConfig) Validate() error {
	if cfg.ParamPollingInterval <= 0 {
		return errors.New("param-polling-interval must be positive")
	}

	if cfg.ExpiryCheckerPollingInterval <= 0 {
		return errors.New("expiry-checker-polling-interval must be positive")
	}

	if cfg.ExpiredDelegationsLimit <= 0 {
		return errors.New("expired-delegations-limit must be positive")
	}

	return nil
}
