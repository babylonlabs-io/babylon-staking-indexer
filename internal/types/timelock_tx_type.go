package types

type TimeLockTxType string

const (
	ExpiredTxType TimeLockTxType = "EXPIRED"
)

func (t TimeLockTxType) String() string {
	return string(t)
}
