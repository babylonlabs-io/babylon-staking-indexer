//go:build manual

package bbnclient

import (
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/pkg"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestMe(t *testing.T) {
	ctx := t.Context()

	// 899651
	cl, err := NewBBNClient(&config.BBNConfig{
		RPCAddr: "https://babylon-rpc.polkachu.com/",
		Timeout: time.Second,
	})
	require.NoError(t, err)

	blockNumber, err := cl.GetLatestBlockNumber(ctx)
	require.NoError(t, err)

	spew.Dump(blockNumber)
}

func TestBBNClient(t *testing.T) {
	rpcAddr := pkg.Getenv("BABYLON_RPC_ADDR", "https://rpc.bsn-devnet.babylonlabs.io/")

	cl, err := NewBBNClient(&config.BBNConfig{
		RPCAddr: rpcAddr,
		Timeout: time.Second,
	})
	require.NoError(t, err)

	params, err := cl.GetAllStakingParams(t.Context())
	require.NoError(t, err)

	spew.Dump(params)
}
