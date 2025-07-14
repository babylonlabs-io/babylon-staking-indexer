package utils

import (
	"fmt"
	"github.com/btcsuite/btcd/wire"
)

// IsTransferTx Transfer transaction is a transaction which:
// - has exactly one input
// - has exactly one output
func IsTransferTx(tx *wire.MsgTx) error {
	if tx == nil {
		return fmt.Errorf("transfer transaction must have cannot be nil")
	}

	if len(tx.TxIn) != 1 {
		return fmt.Errorf("transfer transaction must have exactly one input")
	}

	if len(tx.TxOut) != 1 {
		return fmt.Errorf("transfer transaction must have exactly one output")
	}

	return nil
}
