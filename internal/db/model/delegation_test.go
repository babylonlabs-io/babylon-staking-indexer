package model

import (
	"encoding/hex"
	"testing"

	bbntypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromEventBTCDelegationCreated_StakingOutputIdxOutOfRange(t *testing.T) {
	// Build a minimal valid BTC transaction with 1 output
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 0},
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    10000,
		PkScript: []byte{0x00, 0x14, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14},
	})

	var buf []byte
	w := new(wireBuffer)
	err := tx.Serialize(w)
	require.NoError(t, err)
	buf = w.bytes

	stakingTxHex := hex.EncodeToString(buf)

	tests := []struct {
		name           string
		stakingOutIdx  string
		wantErr        string
	}{
		{
			name:          "output index equals output count",
			stakingOutIdx: "1",
			wantErr:       "staking output index 1 out of range (tx has 1 outputs)",
		},
		{
			name:          "output index far exceeds output count",
			stakingOutIdx: "999",
			wantErr:       "staking output index 999 out of range (tx has 1 outputs)",
		},
		{
			name:          "valid output index 0",
			stakingOutIdx: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &bbntypes.EventBTCDelegationCreated{
				StakingTxHex:       stakingTxHex,
				StakingOutputIndex: tt.stakingOutIdx,
				ParamsVersion:      "0",
				StakingTime:        "1000",
				UnbondingTime:      "100",
				UnbondingTx:        "",
				StakerBtcPkHex:     "aabbccdd",
				StakerAddr:         "bbn1test",
				NewState:           "PENDING",
			}

			result, err := FromEventBTCDelegationCreated(event, 100, 1000)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

// wireBuffer is a simple io.Writer that accumulates bytes for serialization.
type wireBuffer struct {
	bytes []byte
}

func (w *wireBuffer) Write(p []byte) (int, error) {
	w.bytes = append(w.bytes, p...)
	return len(p), nil
}
