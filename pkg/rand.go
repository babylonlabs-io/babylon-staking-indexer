package pkg

import (
	"math/rand"
	"strings"
)

func RandString(n int) string {
	var builder strings.Builder
	builder.Grow(n)

	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for range n {
		letter := letters[rand.Intn(len(letters))] //nolint:gosec
		builder.WriteByte(letter)
	}

	return builder.String()
}
