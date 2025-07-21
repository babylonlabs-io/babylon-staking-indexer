package bbnclient

import (
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMe(t *testing.T) {
	cl, err := NewBBNClient(&config.BBNConfig{
		RPCAddr: "https://rpc.bsn-devnet.babylonlabs.io/",
		Timeout: time.Second,
	})
	require.NoError(t, err)

	params, err := cl.GetAllStakingParams(t.Context())
	require.NoError(t, err)

	spew.Dump(params)
}
