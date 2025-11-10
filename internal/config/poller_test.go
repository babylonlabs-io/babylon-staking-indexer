package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollerConfig_Validate(t *testing.T) {
	t.Run("all required fields set", func(t *testing.T) {
		cfg := &PollerConfig{
			ParamPollingInterval:         1 * time.Minute,
			ExpiryCheckerPollingInterval: 2 * time.Minute,
			ExpiredDelegationsLimit:      100,
			StatsPollingInterval:         3 * time.Minute,
		}
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Equal(t, 3*time.Minute, cfg.StatsPollingInterval)
	})

	t.Run("stats polling interval not set - should use default", func(t *testing.T) {
		cfg := &PollerConfig{
			ParamPollingInterval:         1 * time.Minute,
			ExpiryCheckerPollingInterval: 2 * time.Minute,
			ExpiredDelegationsLimit:      100,
			StatsPollingInterval:         0, // not set
		}
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Equal(t, defaultStatsPollingInterval, cfg.StatsPollingInterval)
		assert.Equal(t, 5*time.Minute, cfg.StatsPollingInterval)
	})

	t.Run("stats polling interval negative - should use default", func(t *testing.T) {
		cfg := &PollerConfig{
			ParamPollingInterval:         1 * time.Minute,
			ExpiryCheckerPollingInterval: 2 * time.Minute,
			ExpiredDelegationsLimit:      100,
			StatsPollingInterval:         -1 * time.Minute, // negative
		}
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Equal(t, defaultStatsPollingInterval, cfg.StatsPollingInterval)
	})

	t.Run("param polling interval not set - should error", func(t *testing.T) {
		cfg := &PollerConfig{
			ParamPollingInterval:         0,
			ExpiryCheckerPollingInterval: 2 * time.Minute,
			ExpiredDelegationsLimit:      100,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "param-polling-interval must be positive")
	})

	t.Run("expiry checker polling interval not set - should error", func(t *testing.T) {
		cfg := &PollerConfig{
			ParamPollingInterval:         1 * time.Minute,
			ExpiryCheckerPollingInterval: 0,
			ExpiredDelegationsLimit:      100,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expiry-checker-polling-interval must be positive")
	})

	t.Run("expired delegations limit not set - should error", func(t *testing.T) {
		cfg := &PollerConfig{
			ParamPollingInterval:         1 * time.Minute,
			ExpiryCheckerPollingInterval: 2 * time.Minute,
			ExpiredDelegationsLimit:      0,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired-delegations-limit must be positive")
	})
}
