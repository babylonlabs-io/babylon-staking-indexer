package bbnclient

import (
	"testing"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/stretchr/testify/require"
	"time"
	"github.com/davecgh/go-spew/spew"
)

func TestMe3(t *testing.T) {
	client, err := NewBBNClient(&config.BBNConfig{
		RPCAddr: "https://rpc.edge-devnet.babylonlabs.io",
		Timeout: time.Minute,
	})
	require.NoError(t, err)

	chainID, err := client.GetChainID(t.Context())
	require.NoError(t, err)

	spew.Dump(chainID)
}
