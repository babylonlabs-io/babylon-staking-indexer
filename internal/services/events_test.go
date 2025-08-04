//go:build integration

package services

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/observability/metrics"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/babylonlabs-io/babylon-staking-indexer/pkg"
	"github.com/babylonlabs-io/babylon-staking-indexer/tests/mocks"
	"github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProcessEvent(t *testing.T) {
	t.Run("retries", func(t *testing.T) {
		ctx := t.Context()

		srv := NewService(nil, nil, nil, nil, nil, nil)
		event := BbnEvent{
			Category: "",
			Event: abcitypes.Event{
				Type: string(types.EventFinalityProviderCreatedType),
			},
		}
		err := srv.processEvent(ctx, event, 0)
		require.ErrorAs(t, err, &retry.Error{})
	})
}

func Test_DelegationExpansion(t *testing.T) {
	// Few important notes:
	//	- this test uses real data from bsn-devnet, in case of error you can always double-check source of truth
	//  - it uses docker only for db, the rest of logic is mocked (including pushes of staking events)
	//  - all subtests are actually parts of the main test, it structured this way so one can easily disable subtest
	//    to debug an error and for clarity
	t.Cleanup(func() {
		resetDatabase(t)
	})

	ctx := t.Context()
	// we can't use in mocks original ctx because it's modified inside context functions
	// this is special variable for clarity to distinguish ctx from other mock.Anything parameters in mock calls
	internalCtx := mock.Anything

	const (
		stakingTxHashHex = "9f43262433597827f3fd584e5df64f59092ec22a3c13bb54d7cc896d4fbc0c63"
		startHeight      = uint32(263034)

		expansionStakingTxHashHex = "791d0b3ca46934444ef3aafbe5094725a6e3a4dcf23be086783d03f1ce2a6fb5"
		expansionStartHeight      = uint32(263048)
	)

	bbn := mocks.NewBbnInterface(t)
	delegationBlock := getBlock(t, 1347)
	bbn.On("GetBlock", internalCtx, pkg.Ptr[int64](1347)).Return(delegationBlock, nil)
	expansionBlock := getBlock(t, 1980)
	bbn.On("GetBlock", internalCtx, pkg.Ptr[int64](1980)).Return(expansionBlock, nil)

	eventConsumer := mocks.NewEventConsumer(t)
	// events related to delegation
	eventConsumer.On("PushActiveStakingEvent", internalCtx, &client.StakingEvent{
		EventType:                 client.ActiveStakingEventType,
		StakingTxHashHex:          stakingTxHashHex,
		StakerBtcPkHex:            "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
		FinalityProviderBtcPksHex: []string{"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab"},
		StakingAmount:             10_000,
		StateHistory:              []string{types.StatePending.String(), types.StateVerified.String()},
	}).Return(nil).Once()
	eventConsumer.On("PushUnbondingStakingEvent", internalCtx, &client.StakingEvent{
		EventType:                 client.UnbondingStakingEventType,
		StakingTxHashHex:          stakingTxHashHex,
		StakerBtcPkHex:            "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
		FinalityProviderBtcPksHex: []string{"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab"},
		StakingAmount:             10_000,
		StateHistory:              []string{types.StatePending.String(), types.StateVerified.String(), types.StateActive.String()},
	}).Return(nil).Once()
	// events related to expansion
	eventConsumer.On("PushActiveStakingEvent", internalCtx, &client.StakingEvent{
		EventType:                 client.ActiveStakingEventType,
		StakingTxHashHex:          expansionStakingTxHashHex,
		StakerBtcPkHex:            "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
		FinalityProviderBtcPksHex: []string{"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab"},
		StakingAmount:             20000,
		StateHistory:              []string{types.StatePending.String(), types.StateVerified.String()},
	}).Return(nil).Once()

	metrics.Init(9999)

	btc := mocks.NewBtcInterface(t)
	btc.On("GetBlockTimestamp", internalCtx, startHeight).Return(int64(1753970681), nil)
	btc.On("GetBlockTimestamp", internalCtx, expansionStartHeight).Return(int64(1753977985), nil)

	btcNotifier := mocks.NewBtcNotifier(t)
	// delegation spend notification registration
	stakingTxHash, err := chainhash.NewHashFromStr(stakingTxHashHex)
	require.NoError(t, err)

	outpoint := &wire.OutPoint{
		Hash: *stakingTxHash,
	}
	btcNotifier.On("RegisterSpendNtfn", outpoint, mock.Anything, startHeight).Return(&chainntnfs.SpendEvent{}, nil).Once()
	// expansion spend notification registration
	expansionStakingTxHash, err := chainhash.NewHashFromStr(expansionStakingTxHashHex)
	require.NoError(t, err)

	expansionOutpoint := &wire.OutPoint{
		Hash: *expansionStakingTxHash,
	}
	btcNotifier.On("RegisterSpendNtfn", expansionOutpoint, mock.Anything, expansionStartHeight).Return(&chainntnfs.SpendEvent{}, nil).Once()

	cfg := &config.Config{
		Poller: config.PollerConfig{
			ExpiredDelegationsLimit: 1000,
		},
	}
	srv := NewService(cfg, testDB, btc, btcNotifier, bbn, eventConsumer)

	items := getBlockEvents(t,
		1347, // creation of delegation
		1348, // covenant signatures and quorum reached events for delegation
		1513, // inclusion proof for delegation
		// blocks that lead to delegation expansion:
		1980, // creation of expansion
		1981, // covenant signatures and quorum reached events for expansion
		2165, // unbonded early event for delegation + inclusion proof for expansion
	)
	for _, item := range items {
		for _, event := range item.events {
			// it's much easier to test this private method instead of setting up the whole event processing pipeline
			err = srv.doProcessEvent(ctx, event, item.blockHeight)
			require.NoError(t, err)
		}
	}

	delegation, err := testDB.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
	require.NoError(t, err)

	expectedDelegation := &model.BTCDelegationDetails{
		StakingTxHashHex:          stakingTxHashHex,
		StakingTxHex:              "0200000001cb4587efc2b409fad9c92619084c021a344498fd16f99d0014315be77be246470100000000ffffffff021027000000000000225120af78b5edbb8558a8b9fc60dd9f14fc04732efd700494a9cca8cc9860f3b17725309e2b0000000000225120b1382c55cafb8d6c7cbf64be5991550b78641e779259ec87b3a0fd680936269100000000",
		StakingTime:               60000,
		StakingAmount:             10_000,
		StakingOutputIdx:          0,
		StakingBTCTimestamp:       1753970681,
		StakerBtcPkHex:            "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
		StakerBabylonAddress:      "bbn1dppj9xellvzrh7x60vft4u8cpkyrvv3camt8ps",
		FinalityProviderBtcPksHex: []string{"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab"},
		StartHeight:               263034,
		EndHeight:                 323034,
		State:                     types.StateUnbonding,
		SubState:                  types.SubStateEarlyUnbonding,
		StateHistory: []model.StateRecord{
			{
				State:        types.StatePending,
				BbnHeight:    1347,
				BbnEventType: "EventBTCDelegationCreated",
			},
			{
				State:        types.StateVerified,
				BbnHeight:    1348,
				BbnEventType: "EventCovenantQuorumReached",
			},
			{
				State:        types.StateActive,
				BbnHeight:    1513,
				BbnEventType: "EventBTCDelegationInclusionProofReceived",
			},
			{
				State:        types.StateUnbonding,
				SubState:     types.SubStateEarlyUnbonding,
				BbnHeight:    2165,
				BbnEventType: "EventBTCDelgationUnbondedEarly",
			},
		},
		ParamsVersion:         0,
		UnbondingTime:         20,
		UnbondingTx:           "0200000001630cbc4f6d89ccd754bb133c2ac22e09594ff65d4e58fdf3277859332426439f0000000000ffffffff01581b000000000000225120371fbc82a29d9b0be545f11444768dc8534f2d24b3c7443f54150a446217a0a400000000",
		UnbondingStartHeight:  263048,
		UnbondingBTCTimestamp: 1753977985,
		CovenantSignatures: []model.CovenantSignature{
			{
				CovenantBtcPkHex: "a5c60c2188e833d39d0fa798ab3f69aa12ed3dd2f3bad659effa252782de3c31",
				SignatureHex:     "42d3d487401721b05e8562c83290954d8b5ef03c232895c95eb2695ecc41b946d6048fbc276610d0aa5c300dd2a81d3a4ee5aced0d69bd9f6852abb0875b064f",
			},
			{
				CovenantBtcPkHex: "ffeaec52a9b407b355ef6967a7ffc15fd6c3fe07de2844d61550475e7a5233e5",
				SignatureHex:     "1d643f3ff8d3bf4146f0bf98a6d5c724706f774345b7790f240e38000c9ee63e8af8d7bd54512b1e747ee3d1758df294dd05e9fbbb8cde5da927fbb6a27dbe89",
			},
			{
				CovenantBtcPkHex: "59d3532148a597a2d05c0395bf5f7176044b1cd312f37701a9b4d0aad70bc5a4",
				SignatureHex:     "4e8e9e381ed8a8440cb7f70b93fcfbef2b563c31bc04faec9777b39a22fd417ae58a5a5d6d5f40a701fabed8c8543d75ab3656f9a4308017be7c42f44fce82f6",
			},
		},
		BTCDelegationCreatedBlock: model.BTCDelegationCreatedBbnBlock{
			Height:    1347,
			Timestamp: 1753970253,
		},
		SlashingTx:               model.SlashingTx{},
		PreviousStakingTxHashHex: "",
	}
	assert.Equal(t, expectedDelegation, delegation)

	t.Run("expiry check", func(t *testing.T) {
		tipHeight := uint64(263657)
		btc.On("GetTipHeight", ctx).Return(tipHeight, nil)

		eventConsumer.On("PushWithdrawableStakingEvent", internalCtx, &client.StakingEvent{
			SchemaVersion:    0,
			EventType:        client.WithdrawableStakingEventType,
			StakingTxHashHex: stakingTxHashHex,
			StakerBtcPkHex:   "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
			FinalityProviderBtcPksHex: []string{
				"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab",
			},
			StakingAmount: 10_000,
			StateHistory: []string{
				types.StatePending.String(),
				types.StateVerified.String(),
				types.StateActive.String(),
				types.StateUnbonding.String(),
			},
		}).Return(nil)

		err = srv.checkExpiry(ctx)
		require.NoError(t, err)

		delegation, err = testDB.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
		require.NoError(t, err)

		// expiry check modified original delegation so it becomes withdrawable
		expectedDelegation.StateHistory = append(expectedDelegation.StateHistory, model.StateRecord{
			State:        types.StateWithdrawable,
			SubState:     types.SubStateEarlyUnbonding,
			BbnHeight:    0,
			BtcHeight:    263068,
			BbnEventType: "",
		})
		expectedDelegation.State = types.StateWithdrawable
		assert.Equal(t, expectedDelegation, delegation)
	})
	t.Run("expansion check", func(t *testing.T) {
		expansion, err := testDB.GetBTCDelegationByStakingTxHash(ctx, expansionStakingTxHashHex)
		require.NoError(t, err)

		expectedExpansion := &model.BTCDelegationDetails{
			StakingTxHashHex:     expansionStakingTxHashHex,
			StakingTxHex:         "0200000002630cbc4f6d89ccd754bb133c2ac22e09594ff65d4e58fdf3277859332426439f0000000000ffffffffd297ea2e005a547023ab97e021ed7552f9ccc9d131283629f84e0c43247e47920100000000ffffffff02204e000000000000225120af78b5edbb8558a8b9fc60dd9f14fc04732efd700494a9cca8cc9860f3b177251f952b0000000000225120b1382c55cafb8d6c7cbf64be5991550b78641e779259ec87b3a0fd680936269100000000",
			StakingTime:          60000,
			StakingAmount:        20000,
			StakingOutputIdx:     0,
			StakingBTCTimestamp:  1753977985,
			StakerBtcPkHex:       "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
			StakerBabylonAddress: "bbn1dppj9xellvzrh7x60vft4u8cpkyrvv3camt8ps",
			FinalityProviderBtcPksHex: []string{
				"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab",
			},
			StartHeight: expansionStartHeight,
			EndHeight:   323048,
			State:       types.StateActive,
			SubState:    "",
			StateHistory: []model.StateRecord{
				{
					State:        types.StatePending,
					SubState:     "",
					BbnHeight:    1980,
					BtcHeight:    0,
					BbnEventType: "EventBTCDelegationCreated",
				},
				{
					State:        types.StateVerified,
					SubState:     "",
					BbnHeight:    1981,
					BtcHeight:    0,
					BbnEventType: "EventCovenantQuorumReached",
				},
				{
					State:        types.StateActive,
					SubState:     "",
					BbnHeight:    2165,
					BtcHeight:    0,
					BbnEventType: "EventBTCDelegationInclusionProofReceived",
				},
			},
			ParamsVersion:         0,
			UnbondingTime:         20,
			UnbondingTx:           "0200000001b56f2acef1033d7886e03bf2dca4e3a6254709e5fbaaf34e443469a43c0b1d790000000000ffffffff016842000000000000225120371fbc82a29d9b0be545f11444768dc8534f2d24b3c7443f54150a446217a0a400000000",
			UnbondingStartHeight:  0,
			UnbondingBTCTimestamp: 0,
			CovenantSignatures: []model.CovenantSignature{
				{
					CovenantBtcPkHex:           "59d3532148a597a2d05c0395bf5f7176044b1cd312f37701a9b4d0aad70bc5a4",
					SignatureHex:               "fdf5f2e73b8156032df1a6726df954f0bb5cbe90d7b7aad0b40ba02b5f74b7ac5a356be2ecba31941dd6a67c4aefab871f85e05e2f1cb3f713d28a2c077816ad",
					StakeExpansionSignatureHex: "441cd3f38115e1147630e04d7c62f12726e2aba20183ad61d29dd80616444f317e24dbecc59301ea258dba25536da722407fd3e564d84709ba48b9ffab2eeb3e",
				},
				{
					CovenantBtcPkHex:           "ffeaec52a9b407b355ef6967a7ffc15fd6c3fe07de2844d61550475e7a5233e5",
					SignatureHex:               "9804cfbcf7c19c0fe1c4a4f802d943138bf61dfae43df7c349e3eb2be49e4acc43ff815ae1fe13b79771a741513a4b5b3255fee2f39d305ae95214c003cff7df",
					StakeExpansionSignatureHex: "150f31cfee01af0308fb97e550f8e00b28753e93a4e604082aa7533874578f513139f6d8182749ec87f79ee0babc5863a06133612af67d0ff412a11120b56555",
				},
				{
					CovenantBtcPkHex:           "a5c60c2188e833d39d0fa798ab3f69aa12ed3dd2f3bad659effa252782de3c31",
					SignatureHex:               "9fe6d62c9bf59f9a39d601aacbaea4da01176527ff4a57575689df68246f85eee9b244fe8f68486b1eaec40edacd665a497c5109315e32567fb53840dccf689d",
					StakeExpansionSignatureHex: "2ef2975d7c9b25514b126cfac147425a45fec41d0d233c1f79e093ab2ce71225293cfac3accbad5180249763d064fc5d39941e018cb891b73884ddee22f8e0d7",
				},
			},
			BTCDelegationCreatedBlock: model.BTCDelegationCreatedBbnBlock{
				Height:    1980,
				Timestamp: 1753976838,
			},
			SlashingTx:               model.SlashingTx{},
			PreviousStakingTxHashHex: stakingTxHashHex, // expansion always refers to its original delegation
		}
		assert.Equal(t, expectedExpansion, expansion)
	})
}

type blockEvents struct {
	blockHeight int64
	events      []BbnEvent
}

// getBlockEvents reads the specified blocks from the testdata directory
// and returns a slice of blockEvents, each containing information about
// the block and its events.
// The returned slice preserves the original block order, no matter the order of input blocks.
func getBlockEvents(t *testing.T, blocks ...int) []blockEvents {
	sort.Ints(blocks)

	var result []blockEvents
	for _, block := range blocks {
		filename := fmt.Sprintf("./testdata/events/%d.json", block)
		buff, err := os.ReadFile(filename)
		require.NoError(t, err)

		var events []abcitypes.Event
		err = json.Unmarshal(buff, &events)
		require.NoError(t, err)

		item := blockEvents{
			blockHeight: int64(block),
		}
		for _, event := range events {
			ev := BbnEvent{
				Category: types.EventCategory(event.Type),
				Event:    event,
			}
			item.events = append(item.events, ev)
		}

		result = append(result, item)
	}

	return result
}

func getBlock(t *testing.T, blockID int64) *ctypes.ResultBlock {
	filename := fmt.Sprintf("./testdata/bbn/%d.json", blockID)

	buff, err := os.ReadFile(filename)
	require.NoError(t, err)

	var block ctypes.ResultBlock
	err = json.Unmarshal(buff, &block)
	require.NoError(t, err)

	return &block
}
