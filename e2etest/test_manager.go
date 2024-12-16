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
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	_ "github.com/babylonlabs-io/babylon/app/params"
	bbnclient "github.com/babylonlabs-io/babylon/client/client"
	bbncfg "github.com/babylonlabs-io/babylon/client/config"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
	queuecfg "github.com/babylonlabs-io/staking-queue-client/config"
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
	eventuallyWaitTimeOut = 40 * time.Second
	eventuallyPollTime    = 1 * time.Second
	regtestParams         = &chaincfg.RegressionNetParams
)

type TestManager struct {
	BitcoindHandler           *BitcoindTestHandler
	BabylonClient             *bbnclient.Client
	BTCClient                 *btcclient.BTCClient
	WalletClient              *rpcclient.Client
	WalletPrivKey             *btcec.PrivateKey
	Config                    *config.Config
	manager                   *container.Manager
	DbClient                  *db.Database
	QueueConsumer             *queuemngr.QueueManager
	ActiveStakingEventChan    <-chan queuecli.QueueMessage
	UnbondingStakingEventChan <-chan queuecli.QueueMessage
}

// StartManager creates a test manager
func StartManager(t *testing.T, numMatureOutputsInWallet uint32, epochInterval uint) *TestManager {
	manager, err := container.NewManager(t)
	require.NoError(t, err)

	btcHandler := NewBitcoindHandler(t, manager)
	bitcoind := btcHandler.Start(t)
	passphrase := "pass"
	_ = btcHandler.CreateWallet("default", passphrase)

	cfg := TestConfig(t)

	cfg.BTC.RPCHost = fmt.Sprintf("127.0.0.1:%s", bitcoind.GetPort("18443/tcp"))

	connCfg, err := cfg.BTC.ToConnConfig()
	require.NoError(t, err)
	rpcclient, err := rpcclient.New(connCfg, nil)
	require.NoError(t, err)
	err = rpcclient.WalletPassphrase(passphrase, 800)
	require.NoError(t, err)

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

	unbondingStakingEventChan, err := queueConsumer.UnbondingStakingQueue.ReceiveMessages()
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
		BitcoindHandler:           btcHandler,
		BabylonClient:             babylonClient,
		BTCClient:                 btcClient,
		WalletClient:              rpcclient,
		WalletPrivKey:             walletPrivKey,
		Config:                    cfg,
		manager:                   manager,
		DbClient:                  dbClient,
		QueueConsumer:             queueConsumer,
		ActiveStakingEventChan:    activeStakingEventChan,
		UnbondingStakingEventChan: unbondingStakingEventChan,
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

func TestConfig(t *testing.T) *config.Config {
	// TODO: ideally this should be setup through config-test.yaml
	cfg := &config.Config{
		BTC: config.BTCConfig{
			RPCHost:              "127.0.0.1:18443",
			RPCUser:              "user",
			RPCPass:              "pass",
			BlockPollingInterval: 30 * time.Second,
			TxPollingInterval:    30 * time.Second,
			BlockCacheSize:       20 * 1024 * 1024, // 20 MB
			MaxRetryTimes:        5,
			RetryInterval:        500 * time.Millisecond,
			NetParams:            "regtest",
		},
		Db: config.DbConfig{
			Address:  "mongodb://localhost:27019/?replicaSet=RS&directConnection=true",
			Username: "root",
			Password: "example",
			DbName:   "babylon-staking-indexer",
		},
		BBN: config.BBNConfig{
			RPCAddr:       "http://localhost:26657",
			Timeout:       20 * time.Second,
			MaxRetryTimes: 3,
			RetryInterval: 1 * time.Second,
		},
		Poller: config.PollerConfig{
			ParamPollingInterval:         1 * time.Second,
			ExpiryCheckerPollingInterval: 1 * time.Second,
			ExpiredDelegationsLimit:      1000,
		},
		Queue: *queuecfg.DefaultQueueConfig(),
		Metrics: config.MetricsConfig{
			Host: "0.0.0.0",
			Port: 2112,
		},
	}
	cfg.Queue.QueueProcessingTimeout = time.Duration(50) * time.Second
	cfg.Queue.ReQueueDelayTime = time.Duration(100) * time.Second

	err := cfg.Validate()
	require.NoError(t, err)

	return cfg
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

func (tm *TestManager) WaitForDelegationStored(t *testing.T,
	ctx context.Context,
	stakingTxHashHex string,
	expectedState types.DelegationState,
	expectedSubState *types.DelegationSubState,
) {
	var storedDelegation *model.BTCDelegationDetails

	// Wait for delegation to be stored in DB and match expected state
	require.Eventually(t, func() bool {
		delegation, err := tm.DbClient.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
		if err != nil || delegation == nil {
			t.Logf("Waiting for delegation %s to be stored, current error: %v", stakingTxHashHex, err)
			return false
		}

		if delegation.State != expectedState {
			t.Logf("Waiting for delegation %s state to be %s, current state: %s",
				stakingTxHashHex, expectedState, delegation.State)
			return false
		}

		if expectedSubState != nil && delegation.SubState != *expectedSubState {
			t.Logf("Waiting for delegation %s sub-state to be %s, current sub-state: %s",
				stakingTxHashHex, expectedSubState, delegation.SubState)
			return false
		}

		storedDelegation = delegation
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	require.NotNil(t, storedDelegation)
	require.Equal(t, stakingTxHashHex, storedDelegation.StakingTxHashHex,
		"Stored delegation hash does not match expected")
	require.Equal(t, expectedState.String(), storedDelegation.State.String(),
		"Stored delegation state does not match expected")
	if expectedSubState != nil {
		require.Equal(t, expectedSubState.String(), storedDelegation.SubState.String(),
			"Stored delegation sub-state does not match expected")
	}
}

func (tm *TestManager) WaitForFinalityProviderStored(t *testing.T, ctx context.Context, fpPKHex string) {
	require.Eventually(t, func() bool {
		fp, err := tm.DbClient.GetFinalityProviderByBtcPk(ctx, fpPKHex)
		if err != nil || fp == nil {
			return false
		}
		return fp != nil && fp.BtcPk == fpPKHex
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) CheckNextActiveStakingEvent(t *testing.T, stakingTxHashHex string) {
	stakingEventBytes := <-tm.ActiveStakingEventChan
	var activeStakingEvent queuecli.StakingEvent
	err := json.Unmarshal([]byte(stakingEventBytes.Body), &activeStakingEvent)
	require.NoError(t, err)

	require.Equal(t, stakingTxHashHex, activeStakingEvent.StakingTxHashHex)

	err = tm.QueueConsumer.ActiveStakingQueue.DeleteMessage(stakingEventBytes.Receipt)
	require.NoError(t, err)
}

func (tm *TestManager) CheckNextUnbondingStakingEvent(t *testing.T, stakingTxHashHex string) {
	unbondingEventBytes := <-tm.UnbondingStakingEventChan
	var unbondingStakingEvent queuecli.StakingEvent
	err := json.Unmarshal([]byte(unbondingEventBytes.Body), &unbondingStakingEvent)
	require.NoError(t, err)

	require.Equal(t, stakingTxHashHex, unbondingStakingEvent.StakingTxHashHex)

	err = tm.QueueConsumer.UnbondingStakingQueue.DeleteMessage(unbondingEventBytes.Receipt)
	require.NoError(t, err)
}
