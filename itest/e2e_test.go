//go:build e2e
// +build e2e

package e2etest

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbndatagen "github.com/babylonlabs-io/babylon/testutil/datagen"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/staking-indexer/cmd/sid/cli"
	"github.com/babylonlabs-io/staking-indexer/config"
	"github.com/babylonlabs-io/staking-indexer/testutils"
	"github.com/babylonlabs-io/staking-indexer/testutils/datagen"
)

func TestBTCScanner(t *testing.T) {
	n := 100
	tm := StartManagerWithNBlocks(t, n, uint64(n))
	defer tm.Stop()

	count, err := tm.BitcoindHandler.GetBlockCount()
	require.NoError(t, err)
	require.Equal(t, n, count)

	k := int(tm.VersionedParams.Versions[0].ConfirmationDepth)

	_ = tm.BitcoindHandler.GenerateBlocks(10)

	tm.WaitForNConfirmations(t, k)
}

func TestQueueConsumer(t *testing.T) {
	// create event consumer
	queueCfg := config.DefaultQueueConfig()
	queueConsumer, err := setupTestQueueConsumer(t, queueCfg)
	require.NoError(t, err)
	stakingChan, err := queueConsumer.StakingQueue.ReceiveMessages()
	require.NoError(t, err)

	defer queueConsumer.Stop()

	n := 1
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	stakingEventList := make([]*queuecli.ActiveStakingEvent, 0)
	for i := 0; i < n; i++ {
		stakingEvent := &queuecli.ActiveStakingEvent{
			EventType:        queuecli.ActiveStakingEventType,
			StakingTxHashHex: hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10)),
		}
		err = queueConsumer.PushStakingEvent(stakingEvent)
		require.NoError(t, err)
		stakingEventList = append(stakingEventList, stakingEvent)
	}

	for i := 0; i < n; i++ {
		stakingEventBytes := <-stakingChan
		var receivedStakingEvent queuecli.ActiveStakingEvent
		err = json.Unmarshal([]byte(stakingEventBytes.Body), &receivedStakingEvent)
		require.NoError(t, err)
		require.Equal(t, stakingEventList[i].StakingTxHashHex, receivedStakingEvent.StakingTxHashHex)
		err = queueConsumer.StakingQueue.DeleteMessage(stakingEventBytes.Receipt)
		require.NoError(t, err)
	}
}

// TestStakingLifeCycle covers the following life cycle
// 1. the staking tx is sent to BTC
// 2. the staking tx is parsed by the indexer
// 3. wait until the staking tx expires
// 4. the subsequent withdraw tx is sent to BTC
// 5. the withdraw tx is identified by the indexer and consumed by the queue
func TestStakingLifeCycle(t *testing.T) {
	// ensure we have UTXOs
	n := 101
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	// generate valid staking tx data
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// TODO: test with multiple system parameters
	sysParams := tm.VersionedParams.Versions[0]
	k := uint64(sysParams.ConfirmationDepth)

	// build, send the staking tx and mine blocks
	stakingTx, testStakingData, stakingInfo := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash := stakingTx.TxHash()
	tm.SendTxWithNConfirmations(t, stakingTx, int(k))

	tm.CheckConfirmedInfoEvent(t, 100, 0)

	// check that the staking tx is already stored
	_ = tm.WaitForStakingTxStored(t, stakingTxHash)

	// check the staking event is received by the queue
	tm.CheckNextStakingEvent(t, stakingTxHash)
	tm.CheckConfirmedInfoEvent(t, 102, uint64(testStakingData.StakingAmount))

	// wait for the staking tx expires
	if uint64(testStakingData.StakingTime) > k {
		tm.BitcoindHandler.GenerateBlocks(int(uint64(testStakingData.StakingTime) - k))
	}

	// build and send withdraw tx and mine blocks
	withdrawSpendInfo, err := stakingInfo.TimeLockPathSpendInfo()
	require.NoError(t, err)

	storedStakingTx, err := tm.Si.GetStakingTxByHash(&stakingTxHash)
	require.NoError(t, err)
	require.NotNil(t, storedStakingTx)
	fundingInfo := &testutils.FundingInfo{
		FundTxOutput:      stakingTx.TxOut[storedStakingTx.StakingOutputIdx],
		FundTxHash:        stakingTxHash,
		FundTxOutputIndex: storedStakingTx.StakingOutputIdx,
		FundTxSpendInfo:   withdrawSpendInfo,
		LockTime:          testStakingData.StakingTime,
		Value:             testStakingData.StakingAmount,
	}
	withdrawTx := testutils.BuildWithdrawTx(
		t,
		tm.WalletPrivKey,
		[]*testutils.FundingInfo{fundingInfo},
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, withdrawTx, int(k))

	// check the withdraw event is received
	tm.CheckNextWithdrawEvent(t, stakingTx.TxHash())
}

// TestWithdrawFromMultipleStakingTxs tests withdrawal from multiple staking tx in
// a single withdrawal tx
func TestWithdrawFromMultipleStakingTxs(t *testing.T) {
	// ensure we have UTXOs
	n := 110
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sysParams := tm.VersionedParams.Versions[0]
	k := uint64(sysParams.ConfirmationDepth)

	// build, send the staking txs and mine blocks
	stakingTx1, testStakingData1, stakingInfo1 := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash1, err := tm.WalletClient.SendRawTransaction(stakingTx1, true)
	stakingTx2, testStakingData2, stakingInfo2 := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash2, err := tm.WalletClient.SendRawTransaction(stakingTx2, true)

	tm.BitcoindHandler.GenerateBlocks(int(k))

	// check that the staking txs are already stored
	_ = tm.WaitForStakingTxStored(t, *stakingTxHash1)
	_ = tm.WaitForStakingTxStored(t, *stakingTxHash2)

	// wait for the staking tx expires
	tm.BitcoindHandler.GenerateBlocks(int(uint64(sysParams.MaxStakingTime) - k))

	// build and send withdraw tx and mine blocks
	withdrawSpendInfo1, err := stakingInfo1.TimeLockPathSpendInfo()
	require.NoError(t, err)
	withdrawSpendInfo2, err := stakingInfo2.TimeLockPathSpendInfo()
	require.NoError(t, err)

	storedStakingTx1, err := tm.Si.GetStakingTxByHash(stakingTxHash1)
	require.NoError(t, err)
	require.NotNil(t, storedStakingTx1)
	storedStakingTx2, err := tm.Si.GetStakingTxByHash(stakingTxHash2)
	require.NoError(t, err)
	require.NotNil(t, storedStakingTx2)

	fundingInfo1 := &testutils.FundingInfo{
		FundTxOutput:      stakingTx1.TxOut[storedStakingTx1.StakingOutputIdx],
		FundTxHash:        *stakingTxHash1,
		FundTxOutputIndex: storedStakingTx1.StakingOutputIdx,
		FundTxSpendInfo:   withdrawSpendInfo1,
		LockTime:          testStakingData1.StakingTime,
		Value:             testStakingData1.StakingAmount,
	}
	fundingInfo2 := &testutils.FundingInfo{
		FundTxOutput:      stakingTx2.TxOut[storedStakingTx2.StakingOutputIdx],
		FundTxHash:        *stakingTxHash2,
		FundTxOutputIndex: storedStakingTx2.StakingOutputIdx,
		FundTxSpendInfo:   withdrawSpendInfo2,
		LockTime:          testStakingData2.StakingTime,
		Value:             testStakingData2.StakingAmount,
	}

	withdrawTx := testutils.BuildWithdrawTx(
		t,
		tm.WalletPrivKey,
		[]*testutils.FundingInfo{fundingInfo1, fundingInfo2},
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, withdrawTx, int(k))

	// check the withdraw events are received
	tm.CheckNextWithdrawEvent(t, *stakingTxHash1)
	tm.CheckNextWithdrawEvent(t, *stakingTxHash2)
}

// TestWithdrawFromMultipleStakingTxs tests withdrawal from multiple unbonding tx in
// a single withdrawal tx
func TestWithdrawFromMultipleUnbondingTxs(t *testing.T) {
	// ensure we have UTXOs
	n := 110
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sysParams := tm.VersionedParams.Versions[0]
	k := uint64(sysParams.ConfirmationDepth)

	// build, send the staking txs and mine blocks
	stakingTx1, testStakingData1, stakingInfo1 := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash1, err := tm.WalletClient.SendRawTransaction(stakingTx1, true)
	require.NoError(t, err)
	stakingTx2, testStakingData2, stakingInfo2 := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash2, err := tm.WalletClient.SendRawTransaction(stakingTx2, true)
	require.NoError(t, err)

	tm.BitcoindHandler.GenerateBlocks(int(k))

	// check that the staking txs are already stored
	storedStakingTx1 := tm.WaitForStakingTxStored(t, *stakingTxHash1)
	storedStakingTx2 := tm.WaitForStakingTxStored(t, *stakingTxHash2)

	// build and send unbonding tx from the previous staking tx
	unbondingTx1, unbondingInfo1 := testutils.BuildUnbondingTx(
		t, sysParams, tm.WalletPrivKey,
		testStakingData1.FinalityProviderKey, testStakingData1.StakingAmount,
		stakingTxHash1, storedStakingTx1.StakingOutputIdx, stakingInfo1, stakingTx1,
		getCovenantPrivKeys(t), regtestParams,
	)
	unbondingTxHash1 := unbondingTx1.TxHash()
	tm.SendTxWithNConfirmations(t, unbondingTx1, 1)

	unbondingTx2, unbondingInfo2 := testutils.BuildUnbondingTx(
		t, sysParams, tm.WalletPrivKey,
		testStakingData2.FinalityProviderKey, testStakingData2.StakingAmount,
		stakingTxHash2, storedStakingTx2.StakingOutputIdx, stakingInfo2, stakingTx2,
		getCovenantPrivKeys(t), regtestParams,
	)
	unbondingTxHash2 := unbondingTx2.TxHash()
	tm.SendTxWithNConfirmations(t, unbondingTx2, 1)
	require.NoError(t, err)

	// wait for the unbonding tx expires
	tm.BitcoindHandler.GenerateBlocks(int(sysParams.UnbondingTime))

	// build and send withdraw tx and mine blocks
	withdrawSpendInfo1, err := unbondingInfo1.TimeLockPathSpendInfo()
	require.NoError(t, err)
	withdrawSpendInfo2, err := unbondingInfo2.TimeLockPathSpendInfo()
	require.NoError(t, err)

	fundingInfo1 := &testutils.FundingInfo{
		FundTxOutput:      unbondingTx1.TxOut[0],
		FundTxHash:        unbondingTxHash1,
		FundTxOutputIndex: 0,
		FundTxSpendInfo:   withdrawSpendInfo1,
		LockTime:          sysParams.UnbondingTime,
		Value:             btcutil.Amount(unbondingTx1.TxOut[0].Value),
	}
	fundingInfo2 := &testutils.FundingInfo{
		FundTxOutput:      unbondingTx2.TxOut[0],
		FundTxHash:        unbondingTxHash2,
		FundTxOutputIndex: 0,
		FundTxSpendInfo:   withdrawSpendInfo2,
		LockTime:          sysParams.UnbondingTime,
		Value:             btcutil.Amount(unbondingTx2.TxOut[0].Value),
	}

	withdrawTx := testutils.BuildWithdrawTx(
		t,
		tm.WalletPrivKey,
		[]*testutils.FundingInfo{fundingInfo1, fundingInfo2},
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, withdrawTx, int(k))

	// check the withdraw event is received
	tm.CheckNextWithdrawEvent(t, *stakingTxHash1)
	tm.CheckNextWithdrawEvent(t, *stakingTxHash2)
	// consume unbonding events
	tm.CheckNextUnbondingEvent(t, unbondingTxHash1)
	tm.CheckNextUnbondingEvent(t, unbondingTxHash2)
}

// TestWithdrawStakingAndUnbondingTxs tests withdrawal from staking and unbonding tx in
// a single withdrawal tx
func TestWithdrawStakingAndUnbondingTxs(t *testing.T) {
	// ensure we have UTXOs
	n := 110
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sysParams := tm.VersionedParams.Versions[0]
	k := uint64(sysParams.ConfirmationDepth)

	// build, send the staking txs and mine blocks
	stakingTx1, testStakingData1, stakingInfo1 := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash1, err := tm.WalletClient.SendRawTransaction(stakingTx1, true)
	require.NoError(t, err)
	stakingTx2, testStakingData2, stakingInfo2 := tm.BuildStakingTx(t, r, sysParams)
	stakingTxHash2, err := tm.WalletClient.SendRawTransaction(stakingTx2, true)
	require.NoError(t, err)

	tm.BitcoindHandler.GenerateBlocks(int(k))

	// check that the staking txs are already stored
	storedStakingTx1 := tm.WaitForStakingTxStored(t, *stakingTxHash1)
	storedStakingTx2 := tm.WaitForStakingTxStored(t, *stakingTxHash2)

	// build unbonding tx 1 from the previous staking tx 1
	unbondingTx1, unbondingInfo1 := testutils.BuildUnbondingTx(
		t, sysParams, tm.WalletPrivKey,
		testStakingData1.FinalityProviderKey, testStakingData1.StakingAmount,
		stakingTxHash1, storedStakingTx1.StakingOutputIdx, stakingInfo1, stakingTx1,
		getCovenantPrivKeys(t), regtestParams,
	)
	unbondingTxHash1, err := tm.WalletClient.SendRawTransaction(unbondingTx1, true)
	require.NoError(t, err)

	// wait for the staking tx expires
	tm.BitcoindHandler.GenerateBlocks(int(uint64(sysParams.MaxStakingTime) - k))

	// build withdrawal from unbonding tx 1 and staking tx 2
	withdrawSpendInfo1, err := unbondingInfo1.TimeLockPathSpendInfo()
	require.NoError(t, err)
	withdrawSpendInfo2, err := stakingInfo2.TimeLockPathSpendInfo()
	require.NoError(t, err)

	fundingInfo1 := &testutils.FundingInfo{
		FundTxOutput:      unbondingTx1.TxOut[0],
		FundTxHash:        *unbondingTxHash1,
		FundTxOutputIndex: 0,
		FundTxSpendInfo:   withdrawSpendInfo1,
		LockTime:          sysParams.UnbondingTime,
		Value:             btcutil.Amount(unbondingTx1.TxOut[0].Value),
	}
	fundingInfo2 := &testutils.FundingInfo{
		FundTxOutput:      stakingTx2.TxOut[storedStakingTx2.StakingOutputIdx],
		FundTxHash:        *stakingTxHash2,
		FundTxOutputIndex: storedStakingTx2.StakingOutputIdx,
		FundTxSpendInfo:   withdrawSpendInfo2,
		LockTime:          testStakingData2.StakingTime,
		Value:             testStakingData2.StakingAmount,
	}

	withdrawTx := testutils.BuildWithdrawTx(
		t,
		tm.WalletPrivKey,
		[]*testutils.FundingInfo{fundingInfo1, fundingInfo2},
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, withdrawTx, int(k))

	// check the withdraw event is received
	// this is expected to receive the withdrawal from
	tm.CheckNextWithdrawEvent(t, *stakingTxHash2)
	tm.CheckNextWithdrawEvent(t, *stakingTxHash1)
	// consume unbonding event
	tm.CheckNextUnbondingEvent(t, *unbondingTxHash1)
}

func TestUnconfirmedTVL(t *testing.T) {
	// ensure we have UTXOs
	n := 101
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	tm.CheckNextUnconfirmedEvent(t, 0, 0)

	// generate valid staking tx data
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// TODO: test with multiple system parameters
	sysParams := tm.VersionedParams.Versions[0]
	k := sysParams.ConfirmationDepth

	// build staking tx
	stakingTx, testStakingData, stakingInfo := tm.BuildStakingTx(t, r, sysParams)
	// send the staking tx and mine 1 block to trigger
	// unconfirmed calculation
	tm.SendTxWithNConfirmations(t, stakingTx, 1)
	tm.CheckNextUnconfirmedEvent(t, 0, uint64(stakingInfo.StakingOutput.Value))

	// confirm the staking tx
	tm.BitcoindHandler.GenerateBlocks(int(k))
	tm.WaitForNConfirmations(t, int(k))
	tm.CheckNextStakingEvent(t, stakingTx.TxHash())
	tm.CheckNextUnconfirmedEvent(t, uint64(stakingInfo.StakingOutput.Value), uint64(stakingInfo.StakingOutput.Value))

	// build and send unbonding tx from the previous staking tx
	stakingTxHash := stakingTx.TxHash()
	unbondingTx, _ := testutils.BuildUnbondingTx(
		t,
		sysParams,
		tm.WalletPrivKey,
		testStakingData.FinalityProviderKey,
		testStakingData.StakingAmount,
		&stakingTxHash,
		1,
		stakingInfo,
		stakingTx,
		getCovenantPrivKeys(t),
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, unbondingTx, 1)
	tm.CheckNextUnconfirmedEvent(t, uint64(stakingInfo.StakingOutput.Value), 0)

	// confirm the unbonding tx
	tm.BitcoindHandler.GenerateBlocks(int(k))
	tm.WaitForNConfirmations(t, int(k))
	tm.CheckNextUnconfirmedEvent(t, 0, 0)
	tm.CheckNextUnbondingEvent(t, unbondingTx.TxHash())
}

// TestIndexerRestart tests following cases upon restart
//  1. it restarts from a previous height before a staking tx is found.
//     We expect the staking event to be replayed
//  2. it restarts exactly from the height it just processed.
//     We expect the staking event not to be replayed
func TestIndexerRestart(t *testing.T) {
	// ensure we have UTXOs
	n := 101
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	// generate valid staking tx data
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sysParams := tm.VersionedParams.Versions[0]
	k := sysParams.ConfirmationDepth
	testStakingData := datagen.GenerateTestStakingData(t, r, sysParams)
	stakingInfo, err := btcstaking.BuildV0IdentifiableStakingOutputs(
		sysParams.Tag,
		tm.WalletPrivKey.PubKey(),
		testStakingData.FinalityProviderKey,
		sysParams.CovenantPks,
		sysParams.CovenantQuorum,
		testStakingData.StakingTime,
		testStakingData.StakingAmount,
		regtestParams,
	)
	require.NoError(t, err)

	// send the staking tx and mine blocks
	require.NoError(t, err)
	stakingTx, err := testutils.CreateTxFromOutputsAndSign(
		tm.WalletClient,
		[]*wire.TxOut{stakingInfo.OpReturnOutput, stakingInfo.StakingOutput},
		1000,
		tm.MinerAddr,
	)
	require.NoError(t, err)
	stakingTxHash := stakingTx.TxHash()
	tm.SendTxWithNConfirmations(t, stakingTx, int(k))

	// check that the staking tx is already stored
	_ = tm.WaitForStakingTxStored(t, stakingTxHash)

	// check the staking event is received by the queue
	tm.CheckNextStakingEvent(t, stakingTxHash)

	// restart from a height before staking tx
	restartedTm := ReStartFromHeight(t, tm, uint64(n))
	defer restartedTm.Stop()

	// check the staking event is replayed
	restartedTm.CheckNextStakingEvent(t, stakingTxHash)

	// restart the testing manager again from last processed height + 1
	restartedTm2 := ReStartFromHeight(t, restartedTm, restartedTm.Si.GetStartHeight())
	defer restartedTm2.Stop()

	// no staking event should be replayed as
	// the indexer starts from a higher height
	restartedTm2.CheckNoStakingEvent(t)
}

// TestStakingUnbondingLifeCycle covers the following life cycle
// 1. the staking tx is sent to BTC
// 2. the staking tx is parsed by the indexer
// 3. the subsequent unbonding tx is sent to BTC
// 4. the unbonding tx is identified by the indexer
// 5. the subsequent withdraw tx is sent to BTC
// 6. the withdraw tx is identified by the indexer
func TestStakingUnbondingLifeCycle(t *testing.T) {
	// ensure we have UTXOs
	n := 101
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	// generate valid staking tx data
	// TODO: test with multiple system parameters
	sysParams := tm.VersionedParams.Versions[0]
	k := uint64(sysParams.ConfirmationDepth)
	testStakingData := getTestStakingData(t)
	stakingInfo, err := btcstaking.BuildV0IdentifiableStakingOutputs(
		sysParams.Tag,
		tm.WalletPrivKey.PubKey(),
		testStakingData.FinalityProviderKey,
		sysParams.CovenantPks,
		sysParams.CovenantQuorum,
		testStakingData.StakingTime,
		testStakingData.StakingAmount,
		regtestParams,
	)
	require.NoError(t, err)

	// send the staking tx and mine blocks
	require.NoError(t, err)
	stakingTx, err := testutils.CreateTxFromOutputsAndSign(
		tm.WalletClient,
		[]*wire.TxOut{stakingInfo.OpReturnOutput, stakingInfo.StakingOutput},
		1000,
		tm.MinerAddr,
	)
	require.NoError(t, err)
	stakingTxHash := stakingTx.TxHash()
	tm.SendTxWithNConfirmations(t, stakingTx, int(k))

	// check that the staking tx is already stored
	_ = tm.WaitForStakingTxStored(t, stakingTxHash)

	// check the staking event is received by the queue
	tm.CheckNextStakingEvent(t, stakingTxHash)

	// build and send unbonding tx from the previous staking tx
	storedStakingTx, err := tm.Si.GetStakingTxByHash(&stakingTxHash)
	require.NoError(t, err)
	require.NotNil(t, storedStakingTx)
	unbondingTx, _ := testutils.BuildUnbondingTx(
		t,
		sysParams,
		tm.WalletPrivKey,
		testStakingData.FinalityProviderKey,
		testStakingData.StakingAmount,
		&stakingTxHash,
		storedStakingTx.StakingOutputIdx,
		stakingInfo,
		stakingTx,
		getCovenantPrivKeys(t),
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, unbondingTx, int(k))

	// check the unbonding tx is already stored
	tm.WaitForUnbondingTxStored(t, unbondingTx.TxHash())

	// check the unbonding event is received
	tm.CheckNextUnbondingEvent(t, unbondingTx.TxHash())

	// wait for the unbonding tx expires
	if uint64(sysParams.UnbondingTime) > k {
		tm.BitcoindHandler.GenerateBlocks(int(uint64(sysParams.UnbondingTime) - k))
	}

	// build and send withdraw tx from the unbonding tx
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		tm.WalletPrivKey.PubKey(),
		[]*btcec.PublicKey{testStakingData.FinalityProviderKey},
		sysParams.CovenantPks,
		sysParams.CovenantQuorum,
		sysParams.UnbondingTime,
		testStakingData.StakingAmount.MulF64(0.9),
		regtestParams,
	)
	require.NoError(t, err)
	withdrawSpendInfo, err := unbondingInfo.TimeLockPathSpendInfo()
	require.NoError(t, err)
	fundingInfo := &testutils.FundingInfo{
		// unbonding tx only has one output
		FundTxOutput:      unbondingTx.TxOut[0],
		FundTxHash:        unbondingTx.TxHash(),
		FundTxOutputIndex: 0,
		FundTxSpendInfo:   withdrawSpendInfo,
		LockTime:          sysParams.UnbondingTime,
		Value:             testStakingData.StakingAmount,
	}
	withdrawTx := testutils.BuildWithdrawTx(
		t,
		tm.WalletPrivKey,
		[]*testutils.FundingInfo{fundingInfo},
		regtestParams,
	)
	tm.SendTxWithNConfirmations(t, withdrawTx, int(k))

	// wait until the indexer identifies the withdraw tx
	tm.WaitForNConfirmations(t, int(k))

	// check the withdraw event is consumed
	tm.CheckNextWithdrawEvent(t, stakingTx.TxHash())
}

// TestTimeBasedCap tests the case where the time-based cap is applied
func TestTimeBasedCap(t *testing.T) {
	// start from the height at which the time-based cap is effective
	n := 110
	tm := StartManagerWithNBlocks(t, n, 100)
	defer tm.Stop()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sysParams := tm.VersionedParams.Versions[1]
	k := uint64(sysParams.ConfirmationDepth)

	// build and send staking tx which should not overflow
	stakingTx, _, _ := tm.BuildStakingTx(t, r, sysParams)
	tm.SendTxWithNConfirmations(t, stakingTx, int(k))
	storedTx := tm.WaitForStakingTxStored(t, stakingTx.TxHash())
	require.False(t, storedTx.IsOverflow)

	// generate blocks so that the height is out of the cap height
	tm.BitcoindHandler.GenerateBlocks(20)
	currentHeight, err := tm.BitcoindHandler.GetBlockCount()
	require.NoError(t, err)
	require.Greater(t, uint64(currentHeight), sysParams.CapHeight)

	// send another staking tx which should be overflow
	stakingTx2, _, _ := tm.BuildStakingTx(t, r, sysParams)
	tm.SendTxWithNConfirmations(t, stakingTx2, int(k))
	storedTx2 := tm.WaitForStakingTxStored(t, stakingTx2.TxHash())
	require.True(t, storedTx2.IsOverflow)
}

func TestBtcHeaders(t *testing.T) {
	r := rand.New(rand.NewSource(10))
	blocksPerRetarget := 2016

	initBlocksQnt := r.Intn(15) + blocksPerRetarget
	btcd, btcClient := StartBtcClientAndBtcHandler(t, initBlocksQnt)

	// from zero height
	infos, err := cli.BtcHeaderInfoList(btcClient, 0, uint64(initBlocksQnt), true)
	require.NoError(t, err)
	require.Equal(t, len(infos), initBlocksQnt+1)

	generatedBlocksQnt := r.Intn(15) + 2
	btcd.GenerateBlocks(generatedBlocksQnt)
	totalBlks := initBlocksQnt + generatedBlocksQnt

	// check from height with interval
	fromBlockHeight := blocksPerRetarget - 1
	toBlockHeight := totalBlks - 2

	infos, err = cli.BtcHeaderInfoList(btcClient, uint64(fromBlockHeight), uint64(toBlockHeight), true)
	require.NoError(t, err)
	require.Equal(t, len(infos), int(toBlockHeight-fromBlockHeight)+1)
	require.EqualValues(t, infos[len(infos)-1].Height, uint64(toBlockHeight))

	// from retarget block
	infos, err = cli.BtcHeaderInfoList(btcClient, uint64(blocksPerRetarget), uint64(totalBlks), true)
	require.NoError(t, err)
	require.Equal(t, len(infos), int(totalBlks-blocksPerRetarget)+1)
}

func getCovenantPrivKeys(t *testing.T) []*btcec.PrivateKey {
	// private keys of the covenant committee which correspond to the public keys in test-params.json
	covenantPrivKeysHex := []string{
		"6a2369c2c9f5cd3c4242834228acdc38b73e5b8930f5f4a9b69e6eaf557e60ed",
	}

	privKeys := make([]*btcec.PrivateKey, len(covenantPrivKeysHex))
	for i, skHex := range covenantPrivKeysHex {
		skBytes, err := hex.DecodeString(skHex)
		require.NoError(t, err)
		sk, _ := btcec.PrivKeyFromBytes(skBytes)
		privKeys[i] = sk
	}

	return privKeys
}

func getTestStakingData(
	t *testing.T,
) *datagen.TestStakingData {
	stakerPrivKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	fpPrivKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	stakingAmount := btcutil.Amount(100000)
	stakingTime := uint16(100)

	return &datagen.TestStakingData{
		StakerKey:           stakerPrivKey.PubKey(),
		FinalityProviderKey: fpPrivKey.PubKey(),
		StakingAmount:       stakingAmount,
		StakingTime:         stakingTime,
	}
}
