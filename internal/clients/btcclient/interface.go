package btcclient

type BtcInterface interface {
	GetBlockCount() (int64, error)
}
