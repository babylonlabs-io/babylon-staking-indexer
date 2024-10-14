package config

import (
	"errors"
	"time"
)

type PollerConfig struct {
	ParamPollingInterval time.Duration `mapstructure:"param-polling-interval"`
}

func (cfg *PollerConfig) Validate() error {
	if cfg.ParamPollingInterval < 0 {
		return errors.New("param-polling-interval must be positive")
	}

	return nil
}
