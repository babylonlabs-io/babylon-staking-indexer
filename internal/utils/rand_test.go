package utils

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestRandomAlphaNum(t *testing.T) {
	// negative case
	str := RandomAlphaNum(-1)
	assert.Empty(t, str)

	// zero length
	str = RandomAlphaNum(0)
	assert.Empty(t, str)

	// usual case
	length := 9
	str = RandomAlphaNum(length)
	assert.Len(t, str, length)
}
