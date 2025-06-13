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
		Right now it's v4.0.0-snapshot.250522.0.20250608144918-51359793bc94 which causes:
			1. there is no image for this version
			2. when babylon publishes v4.0.0 they won't use -snapshot... prefix
	*/
	babylonVersion = "72fb3f243731cfe09006ba31663aeb9ee03f2336" // temporarily using existing docker container, while v4 is not published

	return ImageConfig{
		BitcoindRepository: dockerBitcoindRepository,
		BitcoindVersion:    dockerBitcoindVersionTag,
		BabylonRepository:  dockerBabylondRepository,
		BabylonVersion:     babylonVersion,
	}
}
