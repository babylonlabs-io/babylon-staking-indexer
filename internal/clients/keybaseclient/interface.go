package keybaseclient

import "context"

//go:generate mockery --name=KeybaseInterface --output=../../../tests/mocks --outpkg=mocks --filename=mock_keybase_client.go
type KeybaseInterface interface {
	GetLogoURL(ctx context.Context, identity string) (string, error)
}
