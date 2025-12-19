package bbnclient

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsParamsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("context deadline exceeded"),
			expected: false,
		},
		{
			name:     "params not found - does not exist pattern",
			err:      errors.New("rpc error: code = Unknown desc = version 6 does not exist: the parameters are not found"),
			expected: true,
		},
		{
			name:     "params not found - does not exists (with typo) pattern",
			err:      errors.New("rpc error: code = Unknown desc = version 6 does not exists: the parameters are not found: unknown request"),
			expected: true,
		},
		{
			name:     "params not found - parameters are not found pattern",
			err:      errors.New("parameters are not found"),
			expected: true,
		},
		{
			name:     "params not found - mixed case should not match",
			err:      errors.New("Parameters Are Not Found"),
			expected: false,
		},
		{
			name:     "params not found - partial match does not exist",
			err:      errors.New("the version does not exist in the system"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isParamsNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
