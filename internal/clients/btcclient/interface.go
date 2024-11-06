package btcclient

import (
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/btcsuite/btcd/wire"
)

type Client interface {
	GetTipHeight() (uint64, error)
	GetBlockByHeight(height uint64) (*types.IndexedBlock, error)
	GetBlockHeaderByHeight(height uint64) (*wire.BlockHeader, error)
}
