package btcclient

type Client interface {
	GetTipHeight() (uint64, error)
}
