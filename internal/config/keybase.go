package config

import (
	"fmt"
)

type KeybaseConfig struct {
	URL string `mapstructure:"url"`
}

func (cfg *KeybaseConfig) Validate() error {
	if cfg.URL == "" {
		return fmt.Errorf("Keybase URL must be set")
	}

	return nil
}
