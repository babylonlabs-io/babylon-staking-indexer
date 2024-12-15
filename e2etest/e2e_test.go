package e2etest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	bbndatagen "github.com/babylonlabs-io/babylon/testutil/datagen"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/babylonlabs-io/staking-queue-client/config"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

var (
	defaultEpochInterval = uint(400) //nolint:unused
)

func TestQueueConsumer(t *testing.T) {
	// create event consumer
	queueCfg := config.DefaultQueueConfig()
	queueConsumer, err := setupTestQueueConsumer(t, queueCfg)
	require.NoError(t, err)
	stakingChan, err := queueConsumer.ActiveStakingQueue.ReceiveMessages()
	require.NoError(t, err)

	defer queueConsumer.Stop()

	n := 1
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	stakingEventList := make([]*queuecli.StakingEvent, 0)
	for i := 0; i < n; i++ {
		stakingEvent := queuecli.NewActiveStakingEvent(
			hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10)),
			hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10)),
			[]string{hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10))},
			1000,
		)
		err = queueConsumer.PushActiveStakingEvent(&stakingEvent)
		require.NoError(t, err)
		stakingEventList = append(stakingEventList, &stakingEvent)
	}

	for i := 0; i < n; i++ {
		stakingEventBytes := <-stakingChan
		var receivedStakingEvent queuecli.StakingEvent
		err = json.Unmarshal([]byte(stakingEventBytes.Body), &receivedStakingEvent)
		require.NoError(t, err)
		require.Equal(t, stakingEventList[i].StakingTxHashHex, receivedStakingEvent.StakingTxHashHex)
		err = queueConsumer.ActiveStakingQueue.DeleteMessage(stakingEventBytes.Receipt)
		require.NoError(t, err)
	}
}

// TestActivatingDelegation verifies that a delegation created without an inclusion proof will
// eventually become "active".
// Specifically, that stakingEventWatcher will send a MsgAddBTCDelegationInclusionProof to do so.
func TestActivatingDelegation(t *testing.T) {
	t.Parallel()
	// segwit is activated at height 300. It's necessary for staking/slashing tx
	numMatureOutputs := uint32(300)

	tm := StartManager(t, numMatureOutputs, defaultEpochInterval)
	defer tm.Stop(t)
	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)
	//
	//btcNotifier, err := btcclient.NewNodeBackend(
	//	btcclient.ToBitcoindConfig(tm.Config.BTC),
	//	&chaincfg.RegressionNetParams,
	//	&btcclient.EmptyHintCache{},
	//)
	//require.NoError(t, err)
	//
	//err = btcNotifier.Start()
	//require.NoError(t, err)

	// commonCfg := config.DefaultCommonConfig()

	// set up a finality provider
	_, fpSK := tm.CreateFinalityProvider(t)
	// set up a BTC delegation
	stakingMsgTx, stakingSlashingInfo, _, _ := tm.CreateBTCDelegationWithoutIncl(t, fpSK)
	stakingMsgTxHash := stakingMsgTx.TxHash()

	// send staking tx to Bitcoin node's mempool
	_, err := tm.WalletClient.SendRawTransaction(stakingMsgTx, true)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{&stakingMsgTxHash})) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	mBlock := tm.mineBlock(t)
	require.Equal(t, 2, len(mBlock.Transactions))

	// wait until staking tx is on Bitcoin
	require.Eventually(t, func() bool {
		_, err := tm.WalletClient.GetRawTransaction(&stakingMsgTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// We want to introduce a latency to make sure that we are not trying to submit inclusion proof while the
		// staking tx is not yet K-deep
		time.Sleep(10 * time.Second)
		// Insert k empty blocks to Bitcoin
		btccParamsResp, err := tm.BabylonClient.BTCCheckpointParams()
		if err != nil {
			fmt.Println("Error fetching BTCCheckpointParams:", err)
			return
		}
		for i := 0; i < int(btccParamsResp.Params.BtcConfirmationDepth); i++ {
			tm.mineBlock(t)
		}
		tm.CatchUpBTCLightClient(t)
	}()

	wg.Wait()

	// // make sure we didn't submit any "invalid" incl proof
	// require.Eventually(t, func() bool {
	// 	return promtestutil.ToFloat64(stakingTrackerMetrics.FailedReportedActivateDelegations) == 0
	// }, eventuallyWaitTimeOut, eventuallyPollTime)

	// created delegation lacks inclusion proof, once created it will be in
	// pending status, once convenant signatures are added it will be in verified status,
	// and once the stakingEventWatcher submits MsgAddBTCDelegationInclusionProof it will
	// be in active status
	require.Eventually(t, func() bool {
		resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
		require.NoError(t, err)

		return resp.BtcDelegation.Active
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}
