package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/lightningnetwork/lnd/blockcache"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/chainntnfs/bitcoindnotify"
)

func main() {
	// Define command line flags
	stakingTxHash := "a380abbd28f463c66ad6866ce0bfbd7d60d0947e141f9e9739e8f8d1e88b811a"
	stakingTxHex := "020000000001016b228133c745edce88a06ad6e2ce7f51f33b62e479dd2ff1b758c70c468a69d40000000000fdffffff03102700000000000022512096cc50af360e0a0a7f8f0313cac6bb317f95987594fb1a80bd6ccd2abf1ca5540000000000000000496a476262743000358818f214fcd9d4ccc4296c9079ec25ed440b0df4acc34bedaa76c2c1955a1961550462adbff78ce0694a0643b452f408f3696f64647f0bedbf2a0ee38a9d58ea60764c00000000000022512053bea6a3e87c00ca8cfff0ebcc53acf2ae4590e208f3f4f6b6719c535c166529014079c0c3fa129f745c35f696c539a508b3e84664d9ceeff4d219969fe2547e27490e913d7e7793d9eb42136f10423344cbe79b2171f51a4f17415071212e38c0b9d9630300"
	outputIdx := uint32(0)
	startHeight := uint32(223335)

	btcNotifier, err := NewBTCNotifier()
	if err != nil {
		log.Fatalf("Failed to create BTC notifier: %v", err)
	}

	if err := btcNotifier.Start(); err != nil {
		log.Fatalf("Failed to start BTC notifier: %v", err)
	}

	if !btcNotifier.Started() {
		log.Fatalf("BTC notifier is not started")
	}

	// Parse transaction
	hash, err := chainhash.NewHashFromStr(stakingTxHash)
	if err != nil {
		log.Fatalf("Failed to parse tx hash: %v", err)
	}

	tx, err := deserializeTransaction(stakingTxHex)
	if err != nil {
		log.Fatalf("Failed to deserialize transaction: %v", err)
	}

	// Create outpoint
	outpoint := wire.OutPoint{
		Hash:  *hash,
		Index: outputIdx,
	}

	// Register spend notification
	spendEvent, err := btcNotifier.RegisterSpendNtfn(
		&outpoint,
		tx.TxOut[outputIdx].PkScript,
		startHeight,
	)
	if err != nil {
		log.Fatalf("Failed to register spend notification: %v", err)
	}

	log.Printf("Watching for spend of tx %s output %d\n", hash.String(), outputIdx)
	log.Printf("PkScript: %x\n", tx.TxOut[outputIdx].PkScript)

	// Setup signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for either spend event or interrupt
	select {
	case spend := <-spendEvent.Spend:
		log.Printf("Transaction was spent!")
		log.Printf("Spending tx: %s", spend.SpendingTx.TxHash().String())
		log.Printf("Spent at height: %d", spend.SpendingHeight)
		log.Printf("Spender input index: %d", spend.SpenderInputIndex)
	case <-sigChan:
		log.Println("Received interrupt signal, shutting down...")
	}
}

func deserializeTransaction(txHex string) (*wire.MsgTx, error) {
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	tx := wire.NewMsgTx(2)
	if err := tx.Deserialize(bytes.NewReader(txBytes)); err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %w", err)
	}

	return tx, nil
}

type BTCNotifier struct {
	*bitcoindnotify.BitcoindNotifier
}

func NewBTCNotifier() (*BTCNotifier, error) {

	params := &chaincfg.SigNetParams

	bitcoindCfg := &chain.BitcoindConfig{
		ChainParams:        params,
		Host:               "127.0.0.1:38332",
		User:               "K78L47aCp6NrcLnG0sTD8k5oaNZuwK1m",
		Pass:               "YIr0Y7gMHPofvBDmZYmu2Cm0gR7OGz5x",
		Dialer:             BuildDialer("127.0.0.1:38332"),
		PrunedModeMaxPeers: 10,
		PollingConfig: &chain.PollingConfig{
			BlockPollingInterval:    30 * time.Second,
			TxPollingInterval:       10 * time.Second,
			TxPollingIntervalJitter: 0.5,
		},
	}

	bitcoindConn, err := chain.NewBitcoindConn(bitcoindCfg)
	if err != nil {
		return nil, err
	}

	if err := bitcoindConn.Start(); err != nil {
		return nil, fmt.Errorf("unable to connect to "+
			"bitcoind: %v", err)
	}

	chainNotifier := bitcoindnotify.New(
		bitcoindConn, params, &EmptyHintCache{},
		&EmptyHintCache{}, blockcache.NewBlockCache(1000000),
	)

	return &BTCNotifier{BitcoindNotifier: chainNotifier}, nil
}

func BuildDialer(rpcHost string) func(string) (net.Conn, error) {
	return func(addr string) (net.Conn, error) {
		return net.Dial("tcp", rpcHost)
	}
}

type HintCache interface {
	chainntnfs.SpendHintCache
	chainntnfs.ConfirmHintCache
}

type EmptyHintCache struct{}

var _ HintCache = (*EmptyHintCache)(nil)

func (c *EmptyHintCache) CommitSpendHint(height uint32, spendRequests ...chainntnfs.SpendRequest) error {
	return nil
}
func (c *EmptyHintCache) QuerySpendHint(spendRequest chainntnfs.SpendRequest) (uint32, error) {
	return 0, nil
}
func (c *EmptyHintCache) PurgeSpendHint(spendRequests ...chainntnfs.SpendRequest) error {
	return nil
}

func (c *EmptyHintCache) CommitConfirmHint(height uint32, confRequests ...chainntnfs.ConfRequest) error {
	return nil
}
func (c *EmptyHintCache) QueryConfirmHint(confRequest chainntnfs.ConfRequest) (uint32, error) {
	return 0, nil
}
func (c *EmptyHintCache) PurgeConfirmHint(confRequests ...chainntnfs.ConfRequest) error {
	return nil
}
