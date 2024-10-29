package types

type TimeLockTxType string

const (
	ExpiredTxType        TimeLockTxType = "EXPIRED"
	EarlyUnbondingTxType TimeLockTxType = "EARLY_UNBONDING"
)

func (t TimeLockTxType) String() string {
	return string(t)
}
