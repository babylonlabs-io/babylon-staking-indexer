package btcclient

//go:generate mockery --name=BtcInterface --output=../../../tests/mocks --outpkg=mocks --filename=mock_btc_client.go
type BtcInterface interface {
	GetTipHeight() (uint64, error)
	GetBlockTimestamp(height uint32) (int64, error)
}
