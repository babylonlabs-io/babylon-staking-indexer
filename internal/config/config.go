package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	bbncfg "github.com/babylonlabs-io/babylon/client/config"
	queue "github.com/babylonlabs-io/staking-queue-client/config"
	"github.com/spf13/viper"
)

type Config struct {
	Db      DbConfig          `mapstructure:"db"`
	BTC     BTCConfig         `mapstructure:"btc"`
	BBN     BBNConfig         `mapstructure:"bbn"`
	Poller  PollerConfig      `mapstructure:"poller"`
	Queue   queue.QueueConfig `mapstructure:"queue"`
	Metrics MetricsConfig     `mapstructure:"metrics"`
}

func (cfg *Config) Validate() error {
	if err := cfg.BBN.Validate(); err != nil {
		return err
	}

	if err := cfg.Db.Validate(); err != nil {
		return err
	}

	if err := cfg.BTC.Validate(); err != nil {
		return err
	}

	if err := cfg.Metrics.Validate(); err != nil {
		return err
	}

	if err := cfg.Queue.Validate(); err != nil {
		return err
	}

	if err := cfg.Poller.Validate(); err != nil {
		return err
	}

	return nil
}

// New returns a fully parsed Config object from a given file directory
func New(cfgFile string) (*Config, error) {
	_, err := os.Stat(cfgFile)
	if err != nil {
		return nil, err
	}

	viper.SetConfigFile(cfgFile)

	viper.AutomaticEnv()
	/*
		Below code will replace nested fields in yml into `_` and any `-` into `__` when you try to override this config via env variable
		To give an example:
		1. `some.config.a` can be overriden by `SOME_CONFIG_A`
		2. `some.config-a` can be overriden by `SOME_CONFIG__A`
		This is to avoid using `-` in the environment variable as it's not supported in all os terminal/bash
		Note: vipner package use `.` as delimitter by default. Read more here: https://pkg.go.dev/github.com/spf13/viper#readme-accessing-nested-keys
	*/
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "__"))

	err = viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err = viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if err = cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	bbnCfg := bbncfg.DefaultBabylonConfig()
	cfg := &Config{
		BTC: BTCConfig{
			RPCHost:              "127.0.0.1:18443",
			RPCUser:              "user",
			RPCPass:              "pass",
			BlockPollingInterval: 30 * time.Second,
			TxPollingInterval:    30 * time.Second,
			BlockCacheSize:       20 * 1024 * 1024, // 20 MB
			MaxRetryTimes:        5,
			RetryInterval:        500 * time.Millisecond,
			NetParams:            "regtest",
		},
		Db: DbConfig{
			Address:  "mongodb://localhost:27019/?replicaSet=RS&directConnection=true",
			Username: "root",
			Password: "example",
			DbName:   "babylon-staking-indexer",
		},
		BBN: BBNConfig{
			RPCAddr:       bbnCfg.RPCAddr,
			Timeout:       bbnCfg.Timeout,
			MaxRetryTimes: 3,
			RetryInterval: 1 * time.Second,
		},
		Poller: PollerConfig{
			ParamPollingInterval:         1 * time.Second,
			ExpiryCheckerPollingInterval: 1 * time.Second,
			ExpiredDelegationsLimit:      1000,
		},
		Queue: *queue.DefaultQueueConfig(),
		Metrics: MetricsConfig{
			Host: "0.0.0.0",
			Port: 2112,
		},
	}

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	return cfg
}
