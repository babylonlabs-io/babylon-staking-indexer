package e2etest

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/e2etest/container"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	bbnclient "github.com/babylonlabs-io/babylon/client/client"
	bbncfg "github.com/babylonlabs-io/babylon/client/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var (
	submitterAddrStr = "bbn1eppc73j56382wjn6nnq3quu5eye4pmm087xfdh" //nolint:unused
	babylonTag       = []byte{1, 2, 3, 4}                           //nolint:unused
	babylonTagHex    = hex.EncodeToString(babylonTag)               //nolint:unused

	eventuallyWaitTimeOut = 40 * time.Second
	eventuallyPollTime    = 1 * time.Second
	regtestParams         = &chaincfg.RegressionNetParams
	defaultEpochInterval  = uint(400) //nolint:unused
)

type TestManager struct {
	BitcoindHandler *BitcoindTestHandler
	BabylonClient   *bbnclient.Client
	BTCClient       *btcclient.BTCClient
	WalletClient    *rpcclient.Client
	WalletPrivKey   *btcec.PrivateKey
	Config          *config.Config
	manager         *container.Manager
}

// StartManager creates a test manager
// NOTE: uses btc client with zmq
func StartManager(t *testing.T, numMatureOutputsInWallet uint32, epochInterval uint) *TestManager {
	manager, err := container.NewManager(t)
	require.NoError(t, err)

	btcHandler := NewBitcoindHandler(t, manager)
	bitcoind := btcHandler.Start(t)
	passphrase := "pass"
	_ = btcHandler.CreateWallet("default", passphrase)
	resp := btcHandler.GenerateBlocks(int(numMatureOutputsInWallet))
	minerAddressDecoded, err := btcutil.DecodeAddress(resp.Address, regtestParams)
	require.NoError(t, err)

	cfg := DefaultStakingIndexerConfig()

	cfg.BTC.RPCHost = fmt.Sprintf("127.0.0.1:%s", bitcoind.GetPort("18443/tcp"))

	connCfg, err := cfg.BTC.ToConnConfig()
	require.NoError(t, err)
	rpcclient, err := rpcclient.New(connCfg, nil)
	require.NoError(t, err)
	err = rpcclient.WalletPassphrase(passphrase, 200)
	require.NoError(t, err)
	walletPrivKey, err := rpcclient.DumpPrivKey(minerAddressDecoded)
	require.NoError(t, err)

	var buff bytes.Buffer
	err = regtestParams.GenesisBlock.Header.Serialize(&buff)
	require.NoError(t, err)
	baseHeaderHex := hex.EncodeToString(buff.Bytes())

	pkScript, err := txscript.PayToAddrScript(minerAddressDecoded)
	require.NoError(t, err)

	// start Babylon node

	tmpDir, err := tempDir(t)
	require.NoError(t, err)

	babylond, err := manager.RunBabylondResource(t, tmpDir, baseHeaderHex, hex.EncodeToString(pkScript), epochInterval)
	require.NoError(t, err)

	defaultBbnCfg := bbncfg.DefaultBabylonConfig()

	// create Babylon client
	defaultBbnCfg.KeyDirectory = filepath.Join(tmpDir, "node0", "babylond")
	defaultBbnCfg.Key = "test-spending-key" // keyring to bbn node
	defaultBbnCfg.GasAdjustment = 3.0

	// update port with the dynamically allocated one from docker
	defaultBbnCfg.RPCAddr = fmt.Sprintf("http://localhost:%s", babylond.GetPort("26657/tcp"))
	defaultBbnCfg.GRPCAddr = fmt.Sprintf("https://localhost:%s", babylond.GetPort("9090/tcp"))

	babylonClient, err := bbnclient.New(&defaultBbnCfg, nil)
	require.NoError(t, err)

	// wait until Babylon is ready
	require.Eventually(t, func() bool {
		_, err := babylonClient.CurrentEpoch()
		if err != nil {
			return false
		}
		//log.Infof("Babylon is ready: %v", resp)
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	btcClient, err := btcclient.NewBTCClient(
		&cfg.BTC,
	)
	require.NoError(t, err)

	return &TestManager{
		WalletClient:    rpcclient,
		BabylonClient:   babylonClient,
		BitcoindHandler: btcHandler,
		BTCClient:       btcClient,
		Config:          cfg,
		WalletPrivKey:   walletPrivKey.PrivKey,
		manager:         manager,
	}
}

func (tm *TestManager) Stop(t *testing.T) {
	if tm.BabylonClient.IsRunning() {
		err := tm.BabylonClient.Stop()
		require.NoError(t, err)
	}
}

// mineBlock mines a single block
func (tm *TestManager) mineBlock(t *testing.T) *wire.MsgBlock {
	resp := tm.BitcoindHandler.GenerateBlocks(1)

	hash, err := chainhash.NewHashFromStr(resp.Blocks[0])
	require.NoError(t, err)

	header, err := tm.WalletClient.GetBlock(hash)
	require.NoError(t, err)

	return header
}

func (tm *TestManager) MustGetBabylonSigner() string {
	return tm.BabylonClient.MustGetAddr()
}

func tempDir(t *testing.T) (string, error) {
	tempPath, err := os.MkdirTemp(os.TempDir(), "babylon-test-*")
	if err != nil {
		return "", err
	}

	if err = os.Chmod(tempPath, 0777); err != nil {
		return "", err
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempPath)
	})

	return tempPath, err
}

func DefaultStakingIndexerConfig() *config.Config {
	defaultConfig := config.DefaultConfig()

	// enable emitting extra events for testing
	//defaultConfig.ExtraEventEnabled = true

	// both wallet and node are bitcoind
	defaultConfig.BTC.NetParams = regtestParams.Name

	bitcoindHost := "127.0.0.1:18443"
	bitcoindUser := "user"
	bitcoindPass := "pass"

	defaultConfig.BTC.RPCHost = bitcoindHost
	defaultConfig.BTC.RPCUser = bitcoindUser
	defaultConfig.BTC.RPCPass = bitcoindPass
	defaultConfig.BTC.BlockPollingInterval = 1 * time.Second
	defaultConfig.BTC.TxPollingInterval = 1 * time.Second

	return defaultConfig
}
