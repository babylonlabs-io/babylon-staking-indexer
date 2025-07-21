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
