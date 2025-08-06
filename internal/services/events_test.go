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
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
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
	"time"
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

	// staking params are required for correct spend notification processing
	fixtures := loadDbTestdata(t, "global_params.json")
	collection := mongoDB.Collection(model.GlobalParamsCollection)
	_, err := collection.InsertMany(ctx, fixtures)
	require.NoError(t, err)

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
	stakingTxHash := hashFromString(t, stakingTxHashHex)
	expansionStakingTxHash := hashFromString(t, expansionStakingTxHashHex)

	stakingOutpoint := &wire.OutPoint{
		Hash: *stakingTxHash,
	}
	delegationSpendCh := make(chan *chainntnfs.SpendDetail, 1) // we need unbuffered because there is no reader yet
	delegationSpendCh <- &chainntnfs.SpendDetail{
		SpentOutPoint: stakingOutpoint,
		SpenderTxHash: expansionStakingTxHash,
		SpendingTx: &wire.MsgTx{
			Version: 2,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: *stakingOutpoint,
					SignatureScript:  []byte{},
					Witness: [][]byte{
						{
							0x15, 0x0f, 0x31, 0xcf, 0xee, 0x01, 0xaf, 0x03,
							0x08, 0xfb, 0x97, 0xe5, 0x50, 0xf8, 0xe0, 0x0b,
							0x28, 0x75, 0x3e, 0x93, 0xa4, 0xe6, 0x04, 0x08,
							0x2a, 0xa7, 0x53, 0x38, 0x74, 0x57, 0x8f, 0x51,
							0x31, 0x39, 0xf6, 0xd8, 0x18, 0x27, 0x49, 0xec,
							0x87, 0xf7, 0x9e, 0xe0, 0xba, 0xbc, 0x58, 0x63,
							0xa0, 0x61, 0x33, 0x61, 0x2a, 0xf6, 0x7d, 0x0f,
							0xf4, 0x12, 0xa1, 0x11, 0x20, 0xb5, 0x65, 0x55,
						},
						{
							0x2e, 0xf2, 0x97, 0x5d, 0x7c, 0x9b, 0x25, 0x51,
							0x4b, 0x12, 0x6c, 0xfa, 0xc1, 0x47, 0x42, 0x5a,
							0x45, 0xfe, 0xc4, 0x1d, 0x0d, 0x23, 0x3c, 0x1f,
							0x79, 0xe0, 0x93, 0xab, 0x2c, 0xe7, 0x12, 0x25,
							0x29, 0x3c, 0xfa, 0xc3, 0xac, 0xcb, 0xad, 0x51,
							0x80, 0x24, 0x97, 0x63, 0xd0, 0x64, 0xfc, 0x5d,
							0x39, 0x94, 0x1e, 0x01, 0x8c, 0xb8, 0x91, 0xb7,
							0x38, 0x84, 0xdd, 0xee, 0x22, 0xf8, 0xe0, 0xd7,
						},
						{},
						{
							0xf6, 0x60, 0x0d, 0xd9, 0xce, 0x76, 0x8e, 0xad,
							0x2f, 0x15, 0x8f, 0x75, 0x8f, 0xa3, 0x24, 0xef,
							0x4e, 0xd5, 0xb8, 0xdf, 0x93, 0x86, 0x9d, 0xd4,
							0xce, 0xc1, 0x5c, 0xb5, 0xfd, 0x63, 0x95, 0xa6,
							0xbd, 0x66, 0xf8, 0x41, 0x7d, 0xe7, 0x87, 0xc5,
							0x79, 0xc2, 0x02, 0x0d, 0xe4, 0x90, 0x2d, 0x9f,
							0xfb, 0xd7, 0xc0, 0x6d, 0x0f, 0xf8, 0xe2, 0x88,
							0x47, 0x33, 0x33, 0x05, 0xdd, 0x31, 0xe3, 0x99,
						},
						{
							0x20, 0x3f, 0x8f, 0x44, 0x96, 0xa7, 0x36, 0x7a,
							0x7c, 0x3f, 0xe7, 0x8f, 0x95, 0xc0, 0x84, 0x57,
							0x8b, 0x22, 0x8e, 0x20, 0x32, 0x56, 0x97, 0xcf,
							0xe4, 0x23, 0x93, 0x6b, 0x90, 0x5f, 0x7a, 0xc0,
							0x62, 0xad, 0x20, 0x59, 0xd3, 0x53, 0x21, 0x48,
							0xa5, 0x97, 0xa2, 0xd0, 0x5c, 0x03, 0x95, 0xbf,
							0x5f, 0x71, 0x76, 0x04, 0x4b, 0x1c, 0xd3, 0x12,
							0xf3, 0x77, 0x01, 0xa9, 0xb4, 0xd0, 0xaa, 0xd7,
							0x0b, 0xc5, 0xa4, 0xac, 0x20, 0xa5, 0xc6, 0x0c,
							0x21, 0x88, 0xe8, 0x33, 0xd3, 0x9d, 0x0f, 0xa7,
							0x98, 0xab, 0x3f, 0x69, 0xaa, 0x12, 0xed, 0x3d,
							0xd2, 0xf3, 0xba, 0xd6, 0x59, 0xef, 0xfa, 0x25,
							0x27, 0x82, 0xde, 0x3c, 0x31, 0xba, 0x20, 0xff,
							0xea, 0xec, 0x52, 0xa9, 0xb4, 0x07, 0xb3, 0x55,
							0xef, 0x69, 0x67, 0xa7, 0xff, 0xc1, 0x5f, 0xd6,
							0xc3, 0xfe, 0x07, 0xde, 0x28, 0x44, 0xd6, 0x15,
							0x50, 0x47, 0x5e, 0x7a, 0x52, 0x33, 0xe5, 0xba,
							0x52, 0x9c,
						},
						{
							0xc0, 0x50, 0x92, 0x9b, 0x74, 0xc1, 0xa0, 0x49,
							0x54, 0xb7, 0x8b, 0x4b, 0x60, 0x35, 0xe9, 0x7a,
							0x5e, 0x07, 0x8a, 0x5a, 0x0f, 0x28, 0xec, 0x96,
							0xd5, 0x47, 0xbf, 0xee, 0x9a, 0xce, 0x80, 0x3a,
							0xc0, 0x5c, 0x81, 0x56, 0x21, 0x0f, 0x6f, 0xf8,
							0xb2, 0xa1, 0xde, 0x35, 0x43, 0x14, 0x8b, 0x01,
							0x17, 0x8c, 0x4c, 0xcb, 0xf6, 0x37, 0xdf, 0x25,
							0x4d, 0xf9, 0x17, 0xc2, 0x69, 0x6c, 0xb5, 0x7c,
							0x50, 0x4c, 0x8f, 0x73, 0x0f, 0xda, 0x14, 0xa0,
							0xe3, 0x2a, 0xc4, 0x2c, 0x27, 0x7e, 0x99, 0x89,
							0x46, 0xc9, 0x9a, 0xd0, 0x8b, 0x19, 0xbe, 0xa7,
							0x39, 0x31, 0xd4, 0x04, 0x16, 0x5d, 0x87, 0xc2,
							0xd5,
						},
					},
					Sequence: 4294967295,
				},
				{
					PreviousOutPoint: wire.OutPoint{
						Hash:  *hashFromString(t, "92477e24430c4ef829362831d1c9ccf95275ed21e097ab2370545a002eea97d2"),
						Index: 0,
					},
					SignatureScript: []byte{},
					Witness: [][]byte{
						{
							0x29, 0xbb, 0x12, 0x69, 0x88, 0x52, 0x25, 0x9d,
							0x73, 0x3f, 0x0b, 0x0c, 0xf4, 0x8d, 0x2d, 0xb4,
							0xfd, 0xb7, 0xed, 0xc4, 0xc9, 0xa3, 0xf3, 0x0d,
							0x7a, 0x01, 0x5d, 0x24, 0xa9, 0xdd, 0x00, 0x96,
							0xa6, 0xcc, 0x93, 0x31, 0xf3, 0x22, 0x9f, 0x7c,
							0xb6, 0x5a, 0xa4, 0x05, 0x2a, 0xce, 0x7b, 0xfc,
							0xf0, 0xa9, 0xd5, 0xd4, 0xe6, 0x67, 0x1b, 0x04,
							0x3f, 0xc8, 0xa3, 0x3a, 0x04, 0xf0, 0xbf, 0x93,
						},
					}, // todo fill this field
					Sequence: 4294967295,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 20000,
					PkScript: []byte{
						0x51, 0x20, 0xaf, 0x78, 0xb5, 0xed, 0xbb, 0x85,
						0x58, 0xa8, 0xb9, 0xfc, 0x60, 0xdd, 0x9f, 0x14,
						0xfc, 0x04, 0x73, 0x2e, 0xfd, 0x70, 0x04, 0x94,
						0xa9, 0xcc, 0xa8, 0xcc, 0x98, 0x60, 0xf3, 0xb1,
						0x77, 0x25,
					},
				},
				{
					Value: 2856223,
					PkScript: []byte{
						0x51, 0x20, 0xb1, 0x38, 0x2c, 0x55, 0xca, 0xfb,
						0x8d, 0x6c, 0x7c, 0xbf, 0x64, 0xbe, 0x59, 0x91,
						0x55, 0x0b, 0x78, 0x64, 0x1e, 0x77, 0x92, 0x59,
						0xec, 0x87, 0xb3, 0xa0, 0xfd, 0x68, 0x09, 0x36,
						0x26, 0x91,
					},
				},
			},
			LockTime: 0,
		},
		SpenderInputIndex: 0,
		SpendingHeight:    int32(expansionStartHeight),
	}
	btcNotifier.On("RegisterSpendNtfn", stakingOutpoint, mock.Anything, startHeight).Return(&chainntnfs.SpendEvent{
		Spend: delegationSpendCh,
	}, nil).Once()
	// expansion spend notification registration
	expansionOutpoint := &wire.OutPoint{
		Hash: *expansionStakingTxHash,
	}
	expansionSpendCh := make(chan *chainntnfs.SpendDetail, 1) // same as above - there is no reader yet
	expansionSpendCh <- &chainntnfs.SpendDetail{
		SpentOutPoint:     expansionOutpoint,
		SpenderTxHash:     hashFromString(t, "563a44ea1e7d27cae2c83cd079067c3893bd4653cd6b7cc6649361bed4001397"),
		SpendingTx:        nil,
		SpenderInputIndex: 0,
		SpendingHeight:    263056,
	}
	btcNotifier.On("RegisterSpendNtfn", expansionOutpoint, mock.Anything, expansionStartHeight).Return(&chainntnfs.SpendEvent{}, nil).Once()

	cfg := &config.Config{
		Poller: config.PollerConfig{
			ExpiredDelegationsLimit: 1000,
		},
		// this config is required for correct work of RegisterSpendNtfn
		BTC: config.BTCConfig{
			NetParams: utils.BtcSignet.String(),
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
		State:                     types.StateExpansion,
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
				State:        types.StateExpansion,
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
		// this subtest checks that after checkExpiry(ctx) call (it's called by poller)
		// state of the delegation is unmodified
		tipHeight := uint64(263657)
		btc.On("GetTipHeight", ctx).Return(tipHeight, nil)

		//eventConsumer.On("PushWithdrawableStakingEvent", internalCtx, &client.StakingEvent{
		//	SchemaVersion:    0,
		//	EventType:        client.WithdrawableStakingEventType,
		//	StakingTxHashHex: stakingTxHashHex,
		//	StakerBtcPkHex:   "3f8f4496a7367a7c3fe78f95c084578b228e20325697cfe423936b905f7ac062",
		//	FinalityProviderBtcPksHex: []string{
		//		"c384e26491dfec5e021a292a5f3b9b21e3c7aed611d0ecd3a96fd63b8e7e09ab",
		//	},
		//	StakingAmount: 10_000,
		//	StateHistory: []string{
		//		types.StatePending.String(),
		//		types.StateVerified.String(),
		//		types.StateActive.String(),
		//		types.StateUnbonding.String(),
		//	},
		//}).Return(nil)

		err = srv.checkExpiry(ctx)
		require.NoError(t, err)

		delegation, err = testDB.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
		require.NoError(t, err)

		// expiry check modified original delegation so it becomes withdrawable
		//expectedDelegation.StateHistory = append(expectedDelegation.StateHistory, model.StateRecord{
		//	State:        types.StateWithdrawable,
		//	SubState:     types.SubStateEarlyUnbonding,
		//	BbnHeight:    0,
		//	BtcHeight:    263068,
		//	BbnEventType: "",
		//})
		//expectedDelegation.State = types.StateWithdrawable
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

	// giving some time to process spend notifications, we will catch possible errors in the end
	// when there is unexpected method call or access to uninitialized properties
	time.Sleep(2 * time.Second)
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

func hashFromString(t *testing.T, s string) *chainhash.Hash {
	hash, err := chainhash.NewHashFromStr(s)
	require.NoError(t, err)

	return hash
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
