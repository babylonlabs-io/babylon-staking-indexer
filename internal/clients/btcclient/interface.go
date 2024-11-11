package btcclient

type BtcInterface interface {
	GetTipHeight() (uint64, error)
}
