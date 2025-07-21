package pkg

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestGetenv(t *testing.T) {
	const defaultValue = "default"

	t.Cleanup(func() {
		os.Clearenv()
	})
	t.Run("non existing key", func(t *testing.T) {
		value := Getenv("non-existing-key", defaultValue)
		assert.Equal(t, defaultValue, value)
	})
	t.Run("empty value used instead of default", func(t *testing.T) {
		const key = "key"
		err := os.Setenv(key, "")
		require.NoError(t, err)
		assert.Empty(t, Getenv(key, defaultValue))
	})
	t.Run("ok", func(t *testing.T) {
		const (
			key   = "key"
			value = "value"
		)
		err := os.Setenv(key, value)
		require.NoError(t, err)
		assert.Equal(t, value, Getenv(key, defaultValue))
	})
}
