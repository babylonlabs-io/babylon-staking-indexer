package services

import (
	"testing"

	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_extractScriptFromWitness(t *testing.T) {
	tests := []struct {
		name      string
		tx        *wire.MsgTx
		inputIdx  uint32
		wantErr   string
		wantBytes []byte
	}{
		{
			name:     "no inputs",
			tx:       &wire.MsgTx{TxIn: []*wire.TxIn{}},
			inputIdx: 0,
			wantErr:  "input index 0 out of range (tx has 0 inputs)",
		},
		{
			name: "input index out of range",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{Witness: wire.TxWitness{[]byte("a"), []byte("b")}},
				},
			},
			inputIdx: 5,
			wantErr:  "input index 5 out of range (tx has 1 inputs)",
		},
		{
			name: "witness has 0 elements",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{Witness: wire.TxWitness{}},
				},
			},
			inputIdx: 0,
			wantErr:  "spending tx input 0 has 0 witness elements, expected at least 2",
		},
		{
			name: "witness has 1 element",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{Witness: wire.TxWitness{[]byte("only_one")}},
				},
			},
			inputIdx: 0,
			wantErr:  "spending tx input 0 has 1 witness elements, expected at least 2",
		},
		{
			name: "witness has exactly 2 elements - returns first",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{Witness: wire.TxWitness{[]byte("script"), []byte("sig")}},
				},
			},
			inputIdx:  0,
			wantBytes: []byte("script"),
		},
		{
			name: "witness has 3 elements - returns second to last",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{Witness: wire.TxWitness{[]byte("a"), []byte("script"), []byte("control_block")}},
				},
			},
			inputIdx:  0,
			wantBytes: []byte("script"),
		},
		{
			name: "non-zero input index success",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{Witness: wire.TxWitness{[]byte("a"), []byte("b")}},
					{Witness: wire.TxWitness{[]byte("x"), []byte("target_script"), []byte("z")}},
				},
			},
			inputIdx:  1,
			wantBytes: []byte("target_script"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractScriptFromWitness(tt.tx, tt.inputIdx)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantBytes, got)
			}
		})
	}
}
