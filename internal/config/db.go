package config

import (
	"fmt"
	"net/url"
	"strconv"
)

type DbConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DbName   string `mapstructure:"db-name"`
	Address  string `mapstructure:"address"`
}

func (cfg *DbConfig) Validate() error {
	if cfg.Username == "" {
		return fmt.Errorf("missing db username")
	}

	if cfg.Password == "" {
		return fmt.Errorf("missing db password")
	}

	if cfg.Address == "" {
		return fmt.Errorf("missing db address")
	}

	if cfg.DbName == "" {
		return fmt.Errorf("missing db name")
	}

	u, err := url.Parse(cfg.Address)
	if err != nil {
		return fmt.Errorf("invalid db address: %w", err)
	}

	if u.Scheme != "mongodb" {
		return fmt.Errorf("unsupported db scheme: %s", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("missing host in db address")
	}

	port := u.Port()
	if port == "" {
		return fmt.Errorf("missing port in db address")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port in db address: %w", err)
	}

	if portNum < 1024 || portNum > 65535 {
		return fmt.Errorf("port number must be between 1024 and 65535 (inclusive)")
	}

	return nil
}
