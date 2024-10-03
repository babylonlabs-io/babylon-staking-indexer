package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

type PollerConfig struct {
	Interval time.Duration `mapstructure:"interval"`
	LogLevel string        `mapstructure:"log-level"`
}

func (cfg *PollerConfig) Validate() error {
	if cfg.Interval < 0 {
		return errors.New("poll interval cannot be negative")
	}

	if err := cfg.ValidateServiceLogLevel(); err != nil {
		return err
	}

	return nil
}

func (cfg *PollerConfig) ValidateServiceLogLevel() error {
	// If log level is not set, we don't need to validate it, a default value will be used in service
	if cfg.LogLevel == "" {
		return nil
	}

	if parsedLevel, err := zerolog.ParseLevel(cfg.LogLevel); err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	} else if parsedLevel < zerolog.DebugLevel || parsedLevel > zerolog.FatalLevel {
		return fmt.Errorf("only log levels from debug to fatal are supported")
	}
	return nil
}
