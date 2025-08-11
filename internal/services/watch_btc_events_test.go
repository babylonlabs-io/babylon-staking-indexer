//go:build integration

package services

import (
	"encoding/json"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func Test_handleSpendingUnbondingTransaction(t *testing.T) {
	t.Skip()
	ctx := t.Context()

	fixtures := loadDbTestdata(t, "btc_delegation_details.json")
	collection := mongoDB.Collection(model.BTCDelegationDetailsCollection)
	_, err := collection.InsertMany(ctx, fixtures)
	require.NoError(t, err)

	const stakingTxHashHex = "2e95583042e18617a65800ba917de386d8d1081211948f06fc53566194e9a365"

	spendingTx := &wire.MsgTx{}
	srv := NewService(nil, testDB, nil, nil, nil, nil)
	err = srv.handleSpendingStakingTransaction(ctx, spendingTx, 0, 10, stakingTxHashHex)
	require.NoError(t, err)
}

func loadDbTestdata(t *testing.T, filename string) []any {
	buff, err := os.ReadFile(filepath.Join("testdata/db/", filename))
	require.NoError(t, err)

	var fixtures []any
	err = json.Unmarshal(buff, &fixtures)
	require.NoError(t, err)

	return fixtures
}
