package model

import "github.com/babylonlabs-io/babylon-staking-indexer/internal/types"

type TimeLockDocument struct {
	StakingTxHashHex   string                   `bson:"staking_tx_hash_hex"`
	ExpireHeight       uint32                   `bson:"expire_height"`
	DelegationSubState types.DelegationSubState `bson:"delegation_sub_state"`
}

func NewTimeLockDocument(
	stakingTxHashHex string, expireHeight uint32, subState types.DelegationSubState,
) *TimeLockDocument {
	return &TimeLockDocument{
		StakingTxHashHex:   stakingTxHashHex,
		ExpireHeight:       expireHeight,
		DelegationSubState: subState,
	}
}
