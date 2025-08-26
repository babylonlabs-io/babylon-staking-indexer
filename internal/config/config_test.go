package config

import (
	"testing"
	"time"

	queue "github.com/babylonlabs-io/staking-queue-client/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_OptionalKeybase(t *testing.T) {
	// Test with Keybase config present
	cfg := &Config{
		BBN: BBNConfig{
			RPCAddr:       "http://localhost:26657",
			Timeout:       20 * time.Second,
			MaxRetryTimes: 3,
			RetryInterval: 1 * time.Second,
		},
		Db: DbConfig{
			Username: "test",
			Password: "test",
			Address:  "mongodb://localhost:27017",
			DbName:   "test",
		},
		BTC: BTCConfig{
			RPCHost:              "localhost:8332",
			RPCUser:              "test",
			RPCPass:              "test",
			BlockPollingInterval: 30 * time.Second,
			TxPollingInterval:    10 * time.Second,
			BlockCacheSize:       1024,
			MaxRetryTimes:        5,
			RetryInterval:        500 * time.Millisecond,
			NetParams:            "regtest",
		},
		Keybase: KeybaseConfig{
			Timeout:       15 * time.Second,
			MaxRetryTimes: 3,
			RetryInterval: 1 * time.Second,
		},
		Poller: PollerConfig{
			ParamPollingInterval:         10 * time.Second,
			ExpiryCheckerPollingInterval: 10 * time.Second,
			ExpiredDelegationsLimit:      100,
		},
		Queue: queue.QueueConfig{
			QueueUser:              "test",
			QueuePassword:          "test",
			Url:                    "localhost:5672",
			QueueProcessingTimeout: 5 * time.Second,
			MsgMaxRetryAttempts:    10,
			ReQueueDelayTime:       300 * time.Second,
			QueueType:              "quorum",
		},
		Metrics: MetricsConfig{
			Host: "0.0.0.0",
			Port: 2112,
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)
	assert.NotNil(t, cfg.Keybase)

	// Test with Keybase config absent
	cfg.Keybase = nil
	err = cfg.Validate()
	require.NoError(t, err)
	assert.Nil(t, cfg.Keybase)
}
