package model

import "github.com/babylonlabs-io/babylon-staking-indexer/internal/types"

type TimeLockDocument struct {
	StakingTxHashHex string                   `bson:"_id"` // Primary key
	ExpireHeight     uint32                   `bson:"expire_height"`
	SubState         types.DelegationSubState `bson:"sub_state"`
}

func NewTimeLockDocument(
	stakingTxHashHex string, expireHeight uint32, subState types.DelegationSubState,
) *TimeLockDocument {
	return &TimeLockDocument{
		StakingTxHashHex: stakingTxHashHex,
		ExpireHeight:     expireHeight,
		SubState:         subState,
	}
}
