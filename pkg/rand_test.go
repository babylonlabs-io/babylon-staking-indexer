package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandString(t *testing.T) {
	cases := []int{0, 3, 5, 10}
	for _, length := range cases {
		str := RandString(length)
		assert.Len(t, str, length)
	}
}
