package testutil

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// RandomAlphaNum generates random alphanumeric string
// in case length <= 0 it returns empty string
func RandomAlphaNum(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if length <= 0 {
		return "", fmt.Errorf("length must be greater than 0")
	}

	randomString := make([]byte, length)
	for i := range randomString {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		randomString[i] = charset[num.Int64()]
	}

	return string(randomString), nil
}
