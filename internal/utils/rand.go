package utils

import (
	"math/rand"
)

// RandomAlphaNum generates random alphanumeric string
// in case length <= 0 it returns empty string
func RandomAlphaNum(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if length <= 0 {
		return ""
	}

	randomString := make([]byte, length)
	for i := range randomString {
		randomString[i] = charset[rand.Intn(len(charset))]
	}

	return string(randomString)
}
