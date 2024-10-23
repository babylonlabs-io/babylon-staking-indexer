package model

type TimeLockDocument struct {
	StakingTxHashHex string `bson:"_id"` // Primary key
	ExpireHeight     uint32 `bson:"expire_height"`
	TxType           string `bson:"tx_type"`
}

func NewTimeLockDocument(stakingTxHashHex string, expireHeight uint32, txType string) *TimeLockDocument {
	return &TimeLockDocument{
		StakingTxHashHex: stakingTxHashHex,
		ExpireHeight:     expireHeight,
		TxType:           txType,
	}
}
