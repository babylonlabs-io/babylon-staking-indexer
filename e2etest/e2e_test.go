package e2etest

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	bbndatagen "github.com/babylonlabs-io/babylon/testutil/datagen"
	queuecli "github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/babylonlabs-io/staking-queue-client/config"
	"github.com/stretchr/testify/require"
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
