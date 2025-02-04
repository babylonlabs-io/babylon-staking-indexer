package e2etest

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	bbndatagen "github.com/babylonlabs-io/babylon/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
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

	defer func() {
		err := queueConsumer.Stop()
		require.NoError(t, err)
	}()

	n := rand.Intn(10) + 1
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	stakingEventList := make([]*queuecli.StakingEvent, 0)
	for i := 0; i < n; i++ {
		stakingEvent := queuecli.NewActiveStakingEvent(
			hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10)),
			hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10)),
			[]string{hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10))},
			1000,
			[]string{hex.EncodeToString(bbndatagen.GenRandomByteArray(r, 10))},
		)
		err = queueConsumer.PushActiveStakingEvent(context.TODO(), &stakingEvent)
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

// TestStakingEarlyUnbonding verifies the following state transitions:
// PENDING -> VERIFIED -> ACTIVE -> UNBONDING/EARLY_UNBONDING
// 1. Create BTC delegation without inclusion proof in Babylon node (pre-approval flow)
// 2. Wait for delegation to be PENDING in Indexer DB
// 3. Generate and insert new covenant signature in Babylon node
// 4. Wait for delegation to be VERIFIED in Indexer DB
// 5. Submit inclusion proof to Babylon node
// 6. Wait for delegation to be ACTIVE in Babylon node
// 7. Wait for delegation to be ACTIVE in Indexer DB
// 8. Verify active staking event emitted by Indexer
// 9. Early unbonding on Babylon node
// 10. Wait for delegation to be UNBONDED in Babylon node
// 11. Wait for delegation to be UNBONDING and sub-state to be EARLY_UNBONDING in Indexer DB
// 12. Verify unbonding staking event emitted by Indexer
func TestStakingEarlyUnbonding(t *testing.T) {
	// Segw is activated at height 300. It's necessary for staking/slashing tx
	numMatureOutputs := uint32(300)
	ctx := context.Background()

	tm := StartManager(t, numMatureOutputs, defaultEpochInterval)
	defer tm.Stop(t)

	// Insert all existing BTC headers to babylon node
	tm.CatchUpBTCLightClient(t)

	// Create finality provider in Babylon node
	fpPK, fpSK := tm.CreateFinalityProvider(t)

	// Wait for finality provider to be stored in Indexer DB
	tm.WaitForFinalityProviderStored(t, ctx, fpPK.BtcPk.MarshalHex())

	// Create BTC delegation without inclusion proof in Babylon node
	stakingMsgTx, stakingSlashingInfo, unbondingSlashingInfo, _ := tm.CreateBTCDelegationWithoutIncl(t, fpSK)
	stakingMsgTxHash := stakingMsgTx.TxHash()

	// Wait for delegation to be PENDING in Indexer DB
	tm.WaitForDelegationStored(t, ctx, stakingMsgTxHash.String(), types.StatePending, nil)

	// Generate and insert new covenant signature in Babylon node
	slashingSpendPath, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	unbondingSlashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	stakingOutIdx, err := outIdx(stakingSlashingInfo.StakingTx, stakingSlashingInfo.StakingInfo.StakingOutput)
	require.NoError(t, err)
	tm.addCovenantSig(
		t,
		tm.BabylonClient.MustGetAddr(),
		stakingMsgTx,
		&stakingMsgTxHash,
		fpSK, slashingSpendPath,
		stakingSlashingInfo,
		unbondingSlashingInfo,
		unbondingSlashingPathSpendInfo,
		stakingOutIdx,
	)

	// Wait for delegation to be VERIFIED in Indexer DB
	tm.WaitForDelegationStored(t, ctx, stakingMsgTxHash.String(), types.StateVerified, nil)

	// Send staking tx to Bitcoin node's mempool
	_, err = tm.WalletClient.SendRawTransaction(stakingMsgTx, true)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{&stakingMsgTxHash})) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// Mine a block to make sure the staking tx is on Bitcoin
	mBlock := tm.mineBlock(t)
	require.Equal(t, 2, len(mBlock.Transactions))

	// Get spv proof of the BTC staking tx
	stakingTxInfo := getTxInfo(t, mBlock)

	// Wait until staking tx is on Bitcoin
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

	// Submit inclusion proof to Babylon node
	tm.SubmitInclusionProof(t, stakingMsgTxHash.String(), stakingTxInfo)

	// Wait for delegation to be ACTIVE in Babylon node
	require.Eventually(t, func() bool {
		resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
		if err != nil {
			return false
		}

		return resp.BtcDelegation.Active
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// Wait for delegation to be ACTIVE in Indexer DB
	tm.WaitForDelegationStored(t, ctx, stakingMsgTxHash.String(), types.StateActive, nil)

	// Consume active staking event emitted by Indexer
	tm.CheckNextActiveStakingEvent(t, stakingMsgTxHash.String())

	// Early unbonding on Babylon node
	_, _ = tm.Undelegate(t, stakingSlashingInfo, unbondingSlashingInfo, tm.WalletPrivKey, func() { tm.CatchUpBTCLightClient(t) })

	// Wait for delegation to be UNBONDED in Babylon node
	require.Eventually(t, func() bool {
		resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
		if err != nil {
			return false
		}

		return resp.BtcDelegation.StatusDesc == bstypes.BTCDelegationStatus_UNBONDED.String()
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// Wait for delegation to be UNBONDING and sub-state to be EARLY_UNBONDING in Indexer DB
	expectedSubState := types.SubStateEarlyUnbonding
	tm.WaitForDelegationStored(t, ctx, stakingMsgTxHash.String(), types.StateUnbonding, &expectedSubState)

	// Consume unbonding staking event emitted by Indexer
	tm.CheckNextUnbondingStakingEvent(t, stakingMsgTxHash.String())

	// Verify state history in Indexer DB
	delegation, err := tm.DbClient.GetBTCDelegationByStakingTxHash(ctx, stakingMsgTxHash.String())
	require.NoError(t, err)
	require.NotEmpty(t, delegation.StateHistory, "State history should not be empty")
	require.Equal(t, delegation.StateHistory[0].State, types.StatePending)
	require.Equal(t, delegation.StateHistory[0].BbnEventType, types.EventBTCDelegationCreated.ShortName())
	require.Equal(t, delegation.StateHistory[1].State, types.StateVerified)
	require.Equal(t, delegation.StateHistory[1].BbnEventType, types.EventCovenantQuorumReached.ShortName())
	require.Equal(t, delegation.StateHistory[2].State, types.StateActive)
	require.Equal(t, delegation.StateHistory[2].BbnEventType, types.EventBTCDelegationInclusionProofReceived.ShortName())
	require.Equal(t, delegation.StateHistory[3].State, types.StateUnbonding)
	require.Equal(t, delegation.StateHistory[3].SubState, expectedSubState)
	require.Equal(t, delegation.StateHistory[3].BbnEventType, types.EventBTCDelgationUnbondedEarly.ShortName())
}
