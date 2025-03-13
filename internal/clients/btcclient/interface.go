package btcclient

import "context"

//go:generate mockery --name=BtcInterface --output=../../../tests/mocks --outpkg=mocks --filename=mock_btc_client.go
type BtcInterface interface {
	GetTipHeight(ctx context.Context) (uint64, error)
	GetBlockTimestamp(ctx context.Context, height uint32) (int64, error)
}
