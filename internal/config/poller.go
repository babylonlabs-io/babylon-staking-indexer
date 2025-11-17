package config

import (
	"errors"
	"time"
)

const (
	// defaultStatsPollingInterval is the default interval for stats polling (2 minute)
	defaultStatsPollingInterval = 2 * time.Minute
)

type PollerConfig struct {
	ParamPollingInterval         time.Duration `mapstructure:"param-polling-interval"`
	ExpiryCheckerPollingInterval time.Duration `mapstructure:"expiry-checker-polling-interval"`
	ExpiredDelegationsLimit      uint64        `mapstructure:"expired-delegations-limit"`
	StatsPollingInterval         time.Duration `mapstructure:"stats-polling-interval"`
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

	// Set default for stats polling interval if not configured
	if cfg.StatsPollingInterval <= 0 {
		cfg.StatsPollingInterval = defaultStatsPollingInterval
	}

	return nil
}
