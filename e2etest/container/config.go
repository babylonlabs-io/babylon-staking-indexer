package container

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/testutil"
	"github.com/stretchr/testify/require"
)

// ImageConfig contains all images and their respective tags
// needed for running e2e tests.
type ImageConfig struct {
	BitcoindRepository string
	BitcoindVersion    string
	BabylonRepository  string
	BabylonVersion     string
}

//nolint:deadcode
const (
	dockerBitcoindRepository = "lncm/bitcoind"
	dockerBitcoindVersionTag = "v27.0"
	dockerBabylondRepository = "babylonlabs/babylond"
)

// NewImageConfig returns ImageConfig needed for running e2e test.
func NewImageConfig(t *testing.T) ImageConfig {
	babylonVersion, err := testutil.GetBabylonVersion() //nolint:staticcheck,ineffassign
	require.NoError(t, err)

	/*
		We parse our go.mod and fetch specified version above and use this version to setup docker container.
	*/
	babylonVersion = "0a9cfa63316a05f702241b6f2bb32677a12e2df8" // temporarily using existing docker container, while v4 is not published

	return ImageConfig{
		BitcoindRepository: dockerBitcoindRepository,
		BitcoindVersion:    dockerBitcoindVersionTag,
		BabylonRepository:  dockerBabylondRepository,
		BabylonVersion:     babylonVersion,
	}
}
