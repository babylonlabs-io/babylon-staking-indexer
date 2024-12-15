package e2etest

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/e2etest/container"
	indexerbbnclient "github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/btcclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/services"
	_ "github.com/babylonlabs-io/babylon/app/params"
	bbnclient "github.com/babylonlabs-io/babylon/client/client"
	bbncfg "github.com/babylonlabs-io/babylon/client/config"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/babylonlabs-io/staking-queue-client/queuemngr"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	pv "github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	// submitterAddrStr = "bbn1eppc73j56382wjn6nnq3quu5eye4pmm087xfdh" //nolint:unused
	// babylonTag       = []byte{1, 2, 3, 4}                           //nolint:unused
	// babylonTagHex    = hex.EncodeToString(babylonTag)               //nolint:unused

	eventuallyWaitTimeOut = 40 * time.Second
	eventuallyPollTime    = 1 * time.Second
	regtestParams         = &chaincfg.RegressionNetParams
	// defaultEpochInterval  = uint(400) //nolint:unused
)

type TestManager struct {
	BitcoindHandler        *BitcoindTestHandler
	BabylonClient          *bbnclient.Client
	BTCClient              *btcclient.BTCClient
	WalletClient           *rpcclient.Client
	WalletPrivKey          *btcec.PrivateKey
	Config                 *config.Config
	manager                *container.Manager
	DbClient               *db.Database
	QueueConsumer          *queuemngr.QueueManager
	ActiveStakingEventChan <-chan queuecli.QueueMessage
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
	// resp := btcHandler.GenerateBlocks(int(numMatureOutputsInWallet))
	// minerAddressDecoded, err := btcutil.DecodeAddress(resp.Address, regtestParams)
	// require.NoError(t, err)

	cfg := DefaultStakingIndexerConfig()

	cfg.BTC.RPCHost = fmt.Sprintf("127.0.0.1:%s", bitcoind.GetPort("18443/tcp"))

	connCfg, err := cfg.BTC.ToConnConfig()
	require.NoError(t, err)
	rpcclient, err := rpcclient.New(connCfg, nil)
	require.NoError(t, err)
	err = rpcclient.WalletPassphrase(passphrase, 800)
	require.NoError(t, err)
	// walletPrivKey, err := rpcclient.DumpPrivKey(minerAddressDecoded)
	// require.NoError(t, err)

	walletPrivKey, err := importPrivateKey(btcHandler)
	require.NoError(t, err)
	blocksResponse := btcHandler.GenerateBlocks(int(numMatureOutputsInWallet))

	minerAddressDecoded, err := btcutil.DecodeAddress(blocksResponse.Address, regtestParams)
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
		resp, err := babylonClient.CurrentEpoch()
		if err != nil {
			return false
		}
		fmt.Println(resp)
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	btcClient, err := btcclient.NewBTCClient(
		&cfg.BTC,
	)
	require.NoError(t, err)

	ctx := context.Background()
	dbClient, err := db.New(ctx, cfg.Db)
	require.NoError(t, err)

	queueConsumer, err := queuemngr.NewQueueManager(&cfg.Queue, zap.NewNop())
	require.NoError(t, err)

	// queueConsumer, err := setupTestQueueConsumer(t, &cfg.Queue)
	// require.NoError(t, err)

	btcNotifier, err := btcclient.NewBTCNotifier(
		&cfg.BTC,
		&btcclient.EmptyHintCache{},
	)
	require.NoError(t, err)

	cfg.BBN.RPCAddr = fmt.Sprintf("http://localhost:%s", babylond.GetPort("26657/tcp"))
	bbnClient := indexerbbnclient.NewBBNClient(&cfg.BBN)

	service := services.NewService(cfg, dbClient, btcClient, btcNotifier, bbnClient, queueConsumer)
	require.NoError(t, err)

	// initialize metrics with the metrics port from config
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	activeStakingEventChan, err := queueConsumer.ActiveStakingQueue.ReceiveMessages()
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.StartIndexerSync(ctx)
	}()
	// Wait for the server to start
	time.Sleep(3 * time.Second)

	return &TestManager{
		WalletClient:           rpcclient,
		BabylonClient:          babylonClient,
		BitcoindHandler:        btcHandler,
		BTCClient:              btcClient,
		Config:                 cfg,
		WalletPrivKey:          walletPrivKey,
		manager:                manager,
		ActiveStakingEventChan: activeStakingEventChan,
		DbClient:               dbClient,
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

	defaultConfig.Queue.QueueProcessingTimeout = time.Duration(500) * time.Second
	defaultConfig.Queue.ReQueueDelayTime = time.Duration(300) * time.Second

	return defaultConfig
}

// RetrieveTransactionFromMempool fetches transactions from the mempool for the given hashes
func (tm *TestManager) RetrieveTransactionFromMempool(t *testing.T, hashes []*chainhash.Hash) []*btcutil.Tx {
	var txs []*btcutil.Tx
	for _, txHash := range hashes {
		tx, err := tm.WalletClient.GetRawTransaction(txHash)
		require.NoError(t, err)
		txs = append(txs, tx)
	}

	return txs
}

func (tm *TestManager) CatchUpBTCLightClient(t *testing.T) {
	btcHeight, err := tm.WalletClient.GetBlockCount()
	require.NoError(t, err)

	tipResp, err := tm.BabylonClient.BTCHeaderChainTip()
	require.NoError(t, err)
	btclcHeight := tipResp.Header.Height

	var headers []*wire.BlockHeader
	for i := int(btclcHeight + 1); i <= int(btcHeight); i++ {
		hash, err := tm.WalletClient.GetBlockHash(int64(i))
		require.NoError(t, err)
		header, err := tm.WalletClient.GetBlockHeader(hash)
		require.NoError(t, err)
		headers = append(headers, header)
	}

	// Or with JSON formatting
	configJSON, err := json.MarshalIndent(tm.Config, "", "  ")
	require.NoError(t, err)
	t.Logf("Full Config JSON:\n%s", string(configJSON))

	_, err = tm.InsertBTCHeadersToBabylon(headers)
	require.NoError(t, err)
}

func (tm *TestManager) InsertBTCHeadersToBabylon(headers []*wire.BlockHeader) (*pv.RelayerTxResponse, error) {
	var headersBytes []bbn.BTCHeaderBytes

	for _, h := range headers {
		headersBytes = append(headersBytes, bbn.NewBTCHeaderBytesFromBlockHeader(h))
	}

	msg := btclctypes.MsgInsertHeaders{
		Headers: headersBytes,
		Signer:  tm.MustGetBabylonSigner(),
	}

	return tm.BabylonClient.InsertHeaders(context.Background(), &msg)
}

func importPrivateKey(btcHandler *BitcoindTestHandler) (*btcec.PrivateKey, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	wif, err := btcutil.NewWIF(privKey, regtestParams, true)
	if err != nil {
		return nil, err
	}

	// "combo" allows us to import a key and handle multiple types of btc scripts with a single descriptor command.
	descriptor := fmt.Sprintf("combo(%s)", wif.String())

	// Create the JSON descriptor object.
	descJSON, err := json.Marshal([]map[string]interface{}{
		{
			"desc":      descriptor,
			"active":    true,
			"timestamp": "now", // tells Bitcoind to start scanning from the current blockchain height
			"label":     "test key",
		},
	})

	if err != nil {
		return nil, err
	}

	btcHandler.ImportDescriptors(string(descJSON))

	return privKey, nil
}

func (tm *TestManager) WaitForStakingTxStored(t *testing.T, stakingTxHashHex string) *model.BTCDelegationDetails {
	var storedDelegation model.BTCDelegationDetails
	require.Eventually(t, func() bool {
		x, err := tm.DbClient.GetBTCDelegationByStakingTxHash(context.Background(), stakingTxHashHex)
		if err != nil || x == nil {
			return false
		}

		storedDelegation = *x
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	require.Equal(t, stakingTxHashHex, storedDelegation.StakingTxHashHex)

	return &storedDelegation
}

func (tm *TestManager) CheckNextStakingEvent(t *testing.T, stakingTxHashHex string) {
	stakingEventBytes := <-tm.ActiveStakingEventChan
	var activeStakingEvent queuecli.StakingEvent
	err := json.Unmarshal([]byte(stakingEventBytes.Body), &activeStakingEvent)
	require.NoError(t, err)

	storedStakingTx, err := tm.DbClient.GetBTCDelegationByStakingTxHash(context.Background(), stakingTxHashHex)
	require.NotNil(t, storedStakingTx)
	require.NoError(t, err)
	require.Equal(t, stakingTxHashHex, activeStakingEvent.StakingTxHashHex)
	require.Equal(t, storedStakingTx.StakingTxHashHex, activeStakingEvent.StakingTxHashHex)
	require.Equal(t, storedStakingTx.StakingAmount, activeStakingEvent.StakingAmount)
	require.Equal(t, storedStakingTx.StakerBtcPkHex, activeStakingEvent.StakerBtcPkHex)
	require.Equal(t, storedStakingTx.FinalityProviderBtcPksHex, activeStakingEvent.FinalityProviderBtcPksHex)

	err = tm.QueueConsumer.ActiveStakingQueue.DeleteMessage(stakingEventBytes.Receipt)
	require.NoError(t, err)
}
