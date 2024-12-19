package e2etest

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon-staking-indexer/e2etest/types"
	"github.com/babylonlabs-io/babylon/btcstaking"
	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

var (
	r = rand.New(rand.NewSource(time.Now().Unix()))

	// covenant
	covenantSk, _ = btcec.PrivKeyFromBytes(
		[]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	)
)

func (tm *TestManager) getBTCUnbondingTime(t *testing.T) uint32 {
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)

	return bsParams.Params.UnbondingTimeBlocks
}

func (tm *TestManager) CreateFinalityProvider(t *testing.T) (*bstypes.FinalityProvider, *btcec.PrivateKey) {
	var err error
	signerAddr := tm.BabylonClient.MustGetAddr()
	addr := sdk.MustAccAddressFromBech32(signerAddr)

	fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	btcFp, err := datagen.GenRandomFinalityProviderWithBTCBabylonSKs(r, fpSK, addr)
	require.NoError(t, err)

	/*
		create finality provider
	*/
	commission := sdkmath.LegacyZeroDec()
	msgNewVal := &bstypes.MsgCreateFinalityProvider{
		Addr:        signerAddr,
		Description: &stakingtypes.Description{Moniker: datagen.GenRandomHexStr(r, 10)},
		Commission:  &commission,
		BtcPk:       btcFp.BtcPk,
		Pop:         btcFp.Pop,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgNewVal, nil, nil)
	require.NoError(t, err)

	return btcFp, fpSK
}

func (tm *TestManager) CreateBTCDelegation(
	t *testing.T,
	fpSK *btcec.PrivateKey,
) (*datagen.TestStakingSlashingInfo, *datagen.TestUnbondingSlashingInfo, *btcec.PrivateKey) {
	signerAddr := tm.BabylonClient.MustGetAddr()
	addr := sdk.MustAccAddressFromBech32(signerAddr)

	fpPK := fpSK.PubKey()

	/*
		create BTC delegation
	*/
	// generate staking tx and slashing tx
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	covenantBtcPks, err := bbnPksToBtcPks(bsParams.Params.CovenantPks)
	require.NoError(t, err)
	stakingTimeBlocks := bsParams.Params.MaxStakingTimeBlocks
	// get top UTXO
	topUnspentResult, _, err := tm.getHighUTXOAndSum()
	require.NoError(t, err)
	topUTXO, err := types.NewUTXO(topUnspentResult, regtestParams)
	require.NoError(t, err)
	// staking value
	stakingValue := int64(topUTXO.Amount) / 3

	// generate legitimate BTC del
	stakingMsgTx, stakingSlashingInfo, stakingMsgTxHash := tm.createStakingAndSlashingTx(t, fpSK, bsParams, covenantBtcPks, topUTXO, stakingValue, stakingTimeBlocks)

	// send staking tx to Bitcoin node's mempool
	_, err = tm.WalletClient.SendRawTransaction(stakingMsgTx, true)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{stakingMsgTxHash})) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	mBlock := tm.mineBlock(t)
	require.Equal(t, 2, len(mBlock.Transactions))

	// wait until staking tx is on Bitcoin
	require.Eventually(t, func() bool {
		_, err := tm.WalletClient.GetRawTransaction(stakingMsgTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	// get spv proof of the BTC staking tx
	stakingTxInfo := getTxInfo(t, mBlock)

	// insert k empty blocks to Bitcoin
	btccParamsResp, err := tm.BabylonClient.BTCCheckpointParams()
	require.NoError(t, err)
	btccParams := btccParamsResp.Params
	for i := 0; i < int(btccParams.BtcConfirmationDepth); i++ {
		tm.mineBlock(t)
	}

	stakingOutIdx, err := outIdx(stakingSlashingInfo.StakingTx, stakingSlashingInfo.StakingInfo.StakingOutput)
	require.NoError(t, err)

	// create PoP
	pop, err := bstypes.NewPoPBTC(addr, tm.WalletPrivKey)
	require.NoError(t, err)
	slashingSpendPath, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	// generate proper delegator sig
	require.NoError(t, err)

	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		tm.WalletPrivKey,
	)
	require.NoError(t, err)

	// Generate all data necessary for unbonding
	unbondingSlashingInfo, unbondingSlashingPathSpendInfo, unbondingTxBytes, slashingTxSig := tm.createUnbondingData(
		t,
		fpPK,
		bsParams,
		covenantBtcPks,
		stakingSlashingInfo,
		stakingMsgTxHash,
		stakingOutIdx,
		stakingTimeBlocks,
	)

	tm.CatchUpBTCLightClient(t)

	// 	Build a message to send
	// submit BTC delegation to Babylon
	msgBTCDel := &bstypes.MsgCreateBTCDelegation{
		StakerAddr:   signerAddr,
		Pop:          pop,
		BtcPk:        bbn.NewBIP340PubKeyFromBTCPK(tm.WalletPrivKey.PubKey()),
		FpBtcPkList:  []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		StakingTime:  stakingTimeBlocks,
		StakingValue: stakingValue,
		StakingTx:    stakingTxInfo.Transaction,
		StakingTxInclusionProof: &bstypes.InclusionProof{
			Key:   stakingTxInfo.Key,
			Proof: stakingTxInfo.Proof,
		},
		SlashingTx:           stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig: delegatorSig,
		// Unbonding related data
		UnbondingTime:                 tm.getBTCUnbondingTime(t),
		UnbondingTx:                   unbondingTxBytes,
		UnbondingValue:                unbondingSlashingInfo.UnbondingInfo.UnbondingOutput.Value,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: slashingTxSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgBTCDel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgCreateBTCDelegation")

	// generate and insert new covenant signature, to activate the BTC delegation
	tm.addCovenantSig(
		t,
		signerAddr,
		stakingMsgTx,
		stakingMsgTxHash,
		fpSK, slashingSpendPath,
		stakingSlashingInfo,
		unbondingSlashingInfo,
		unbondingSlashingPathSpendInfo,
		stakingOutIdx,
	)

	return stakingSlashingInfo, unbondingSlashingInfo, tm.WalletPrivKey
}

func (tm *TestManager) CreateBTCDelegationWithoutIncl(
	t *testing.T,
	fpSK *btcec.PrivateKey,
) (*wire.MsgTx, *datagen.TestStakingSlashingInfo, *datagen.TestUnbondingSlashingInfo, *btcec.PrivateKey) {
	signerAddr := tm.BabylonClient.MustGetAddr()
	addr := sdk.MustAccAddressFromBech32(signerAddr)

	fpPK := fpSK.PubKey()

	/*
		create BTC delegation
	*/
	// generate staking tx and slashing tx
	bsParams, err := tm.BabylonClient.BTCStakingParams()
	require.NoError(t, err)
	covenantBtcPks, err := bbnPksToBtcPks(bsParams.Params.CovenantPks)
	require.NoError(t, err)
	stakingTimeBlocks := bsParams.Params.MaxStakingTimeBlocks
	// get top UTXO
	topUnspentResult, _, err := tm.getHighUTXOAndSum()
	require.NoError(t, err)
	topUTXO, err := types.NewUTXO(topUnspentResult, regtestParams)
	require.NoError(t, err)
	// staking value
	stakingValue := int64(topUTXO.Amount) / 3

	// generate legitimate BTC del
	stakingMsgTx, stakingSlashingInfo, stakingMsgTxHash := tm.createStakingAndSlashingTx(t, fpSK, bsParams, covenantBtcPks, topUTXO, stakingValue, stakingTimeBlocks)

	stakingOutIdx, err := outIdx(stakingSlashingInfo.StakingTx, stakingSlashingInfo.StakingInfo.StakingOutput)
	require.NoError(t, err)

	// create PoP
	pop, err := bstypes.NewPoPBTC(addr, tm.WalletPrivKey)
	require.NoError(t, err)
	slashingSpendPath, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	// generate proper delegator sig
	require.NoError(t, err)

	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		tm.WalletPrivKey,
	)
	require.NoError(t, err)

	// Generate all data necessary for unbonding
	unbondingSlashingInfo, _, unbondingTxBytes, slashingTxSig := tm.createUnbondingData(
		t,
		fpPK,
		bsParams,
		covenantBtcPks,
		stakingSlashingInfo,
		stakingMsgTxHash,
		stakingOutIdx,
		stakingTimeBlocks,
	)

	var stakingTxBuf bytes.Buffer
	err = stakingMsgTx.Serialize(&stakingTxBuf)
	require.NoError(t, err)

	// submit BTC delegation to Babylon
	msgBTCDel := &bstypes.MsgCreateBTCDelegation{
		StakerAddr:              signerAddr,
		Pop:                     pop,
		BtcPk:                   bbn.NewBIP340PubKeyFromBTCPK(tm.WalletPrivKey.PubKey()),
		FpBtcPkList:             []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		StakingTime:             stakingTimeBlocks,
		StakingValue:            stakingValue,
		StakingTx:               stakingTxBuf.Bytes(),
		StakingTxInclusionProof: nil,
		SlashingTx:              stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:    delegatorSig,
		// Unbonding related data
		UnbondingTime:                 uint32(tm.getBTCUnbondingTime(t)),
		UnbondingTx:                   unbondingTxBytes,
		UnbondingValue:                unbondingSlashingInfo.UnbondingInfo.UnbondingOutput.Value,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: slashingTxSig,
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgBTCDel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgCreateBTCDelegation")

	return stakingMsgTx, stakingSlashingInfo, unbondingSlashingInfo, tm.WalletPrivKey
}

func (tm *TestManager) createStakingAndSlashingTx(
	t *testing.T, fpSK *btcec.PrivateKey,
	bsParams *bstypes.QueryParamsResponse,
	covenantBtcPks []*btcec.PublicKey,
	topUTXO *types.UTXO,
	stakingValue int64,
	stakingTimeBlocks uint32,
) (*wire.MsgTx, *datagen.TestStakingSlashingInfo, *chainhash.Hash) {
	// generate staking tx and slashing tx
	fpPK := fpSK.PubKey()

	// generate legitimate BTC del
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithOutPoint(
		r,
		t,
		regtestParams,
		topUTXO.GetOutPoint(),
		tm.WalletPrivKey,
		[]*btcec.PublicKey{fpPK},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		uint16(stakingTimeBlocks),
		stakingValue,
		bsParams.Params.SlashingPkScript,
		bsParams.Params.SlashingRate,
		uint16(tm.getBTCUnbondingTime(t)),
	)
	// sign staking tx and overwrite the staking tx to the signed version
	// NOTE: the tx hash has changed here since stakingMsgTx is pre-segwit
	stakingMsgTx, signed, err := tm.WalletClient.SignRawTransactionWithWallet(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)
	require.True(t, signed)
	// overwrite staking tx
	stakingSlashingInfo.StakingTx = stakingMsgTx
	// get signed staking tx hash
	stakingMsgTxHash1 := stakingSlashingInfo.StakingTx.TxHash()
	stakingMsgTxHash := &stakingMsgTxHash1
	t.Logf("signed staking tx hash: %s", stakingMsgTxHash.String())

	// change outpoint tx hash of slashing tx to the txhash of the signed staking tx
	slashingMsgTx, err := stakingSlashingInfo.SlashingTx.ToMsgTx()
	require.NoError(t, err)
	slashingMsgTx.TxIn[0].PreviousOutPoint.Hash = stakingSlashingInfo.StakingTx.TxHash()
	// update slashing tx
	stakingSlashingInfo.SlashingTx, err = bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)

	return stakingMsgTx, stakingSlashingInfo, stakingMsgTxHash
}

func (tm *TestManager) createUnbondingData(
	t *testing.T,
	fpPK *btcec.PublicKey,
	bsParams *bstypes.QueryParamsResponse,
	covenantBtcPks []*btcec.PublicKey,
	stakingSlashingInfo *datagen.TestStakingSlashingInfo,
	stakingMsgTxHash *chainhash.Hash,
	stakingOutIdx uint32,
	stakingTimeBlocks uint32,
) (*datagen.TestUnbondingSlashingInfo, *btcstaking.SpendInfo, []byte, *bbn.BIP340Signature) {
	fee := int64(1000)
	unbondingValue := stakingSlashingInfo.StakingInfo.StakingOutput.Value - fee
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		regtestParams,
		tm.WalletPrivKey,
		[]*btcec.PublicKey{fpPK},
		covenantBtcPks,
		bsParams.Params.CovenantQuorum,
		wire.NewOutPoint(stakingMsgTxHash, stakingOutIdx),
		uint16(stakingTimeBlocks),
		unbondingValue,
		bsParams.Params.SlashingPkScript,
		bsParams.Params.SlashingRate,
		uint16(tm.getBTCUnbondingTime(t)),
	)
	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)

	unbondingSlashingPathSpendInfo, err := unbondingSlashingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	slashingTxSig, err := unbondingSlashingInfo.SlashingTx.Sign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding tx
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		tm.WalletPrivKey,
	)
	require.NoError(t, err)

	return unbondingSlashingInfo, unbondingSlashingPathSpendInfo, unbondingTxBytes, slashingTxSig
}

func (tm *TestManager) addCovenantSig(
	t *testing.T,
	signerAddr string,
	stakingMsgTx *wire.MsgTx,
	stakingMsgTxHash *chainhash.Hash,
	fpSK *btcec.PrivateKey,
	slashingSpendPath *btcstaking.SpendInfo,
	stakingSlashingInfo *datagen.TestStakingSlashingInfo,
	unbondingSlashingInfo *datagen.TestUnbondingSlashingInfo,
	unbondingSlashingPathSpendInfo *btcstaking.SpendInfo,
	stakingOutIdx uint32,
) {
	// TODO: Make this handle multiple covenant signatures
	fpEncKey, err := asig.NewEncryptionKeyFromBTCPK(fpSK.PubKey())
	require.NoError(t, err)
	covenantSig, err := stakingSlashingInfo.SlashingTx.EncSign(
		stakingMsgTx,
		stakingOutIdx,
		slashingSpendPath.GetPkScriptPath(),
		covenantSk,
		fpEncKey,
	)
	require.NoError(t, err)
	// TODO: Add covenant sigs for all covenants
	// add covenant sigs
	// covenant Schnorr sig on unbonding tx
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTxCovenantSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
	)
	require.NoError(t, err)
	covenantUnbondingSig := bbn.NewBIP340SignatureFromBTCSig(unbondingTxCovenantSchnorrSig)
	// covenant adaptor sig on unbonding slashing tx
	require.NoError(t, err)
	covenantSlashingSig, err := unbondingSlashingInfo.SlashingTx.EncSign(
		unbondingSlashingInfo.UnbondingTx,
		0, // Only one output in the unbonding transaction
		unbondingSlashingPathSpendInfo.GetPkScriptPath(),
		covenantSk,
		fpEncKey,
	)
	require.NoError(t, err)
	msgAddCovenantSig := &bstypes.MsgAddCovenantSigs{
		Signer:                  signerAddr,
		Pk:                      bbn.NewBIP340PubKeyFromBTCPK(covenantSk.PubKey()),
		StakingTxHash:           stakingMsgTxHash.String(),
		SlashingTxSigs:          [][]byte{covenantSig.MustMarshal()},
		UnbondingTxSig:          covenantUnbondingSig,
		SlashingUnbondingTxSigs: [][]byte{covenantSlashingSig.MustMarshal()},
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgAddCovenantSig, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted covenant signature")
}

func (tm *TestManager) Undelegate(
	t *testing.T,
	stakingSlashingInfo *datagen.TestStakingSlashingInfo,
	unbondingSlashingInfo *datagen.TestUnbondingSlashingInfo,
	delSK *btcec.PrivateKey,
	catchUpLightClientFunc func()) (*datagen.TestUnbondingSlashingInfo, *schnorr.Signature) {
	signerAddr := tm.BabylonClient.MustGetAddr()

	// TODO: This generates unbonding tx signature, move it to undelegate
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	// the only input to unbonding tx is the staking tx
	stakingOutIdx, err := outIdx(unbondingSlashingInfo.UnbondingTx, unbondingSlashingInfo.UnbondingInfo.UnbondingOutput)
	require.NoError(t, err)
	unbondingTxSchnorrSig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingSlashingInfo.UnbondingTx,
		stakingSlashingInfo.StakingTx,
		stakingOutIdx,
		unbondingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	var unbondingTxBuf bytes.Buffer
	err = unbondingSlashingInfo.UnbondingTx.Serialize(&unbondingTxBuf)
	require.NoError(t, err)

	resp, err := tm.BabylonClient.BTCDelegation(stakingSlashingInfo.StakingTx.TxHash().String())
	require.NoError(t, err)
	covenantSigs := resp.BtcDelegation.UndelegationResponse.CovenantUnbondingSigList
	witness, err := unbondingPathSpendInfo.CreateUnbondingPathWitness(
		[]*schnorr.Signature{covenantSigs[0].Sig.MustToBTCSig()},
		unbondingTxSchnorrSig,
	)
	require.NoError(t, err)
	unbondingSlashingInfo.UnbondingTx.TxIn[0].Witness = witness

	// send unbonding tx to Bitcoin node's mempool
	unbondingTxHash, err := tm.WalletClient.SendRawTransaction(unbondingSlashingInfo.UnbondingTx, true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, err := tm.WalletClient.GetRawTransaction(unbondingTxHash)
		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)
	t.Logf("submitted unbonding tx with hash %s", unbondingTxHash.String())

	// mine a block with this tx, and insert it to Bitcoin
	require.Eventually(t, func() bool {
		return len(tm.RetrieveTransactionFromMempool(t, []*chainhash.Hash{unbondingTxHash})) == 1
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	mBlock := tm.mineBlock(t)
	require.Equal(t, 2, len(mBlock.Transactions))

	catchUpLightClientFunc()

	unbondingTxInfo := getTxInfo(t, mBlock)
	msgUndel := &bstypes.MsgBTCUndelegate{
		Signer:          signerAddr,
		StakingTxHash:   stakingSlashingInfo.StakingTx.TxHash().String(),
		StakeSpendingTx: unbondingTxBuf.Bytes(),
		StakeSpendingTxInclusionProof: &bstypes.InclusionProof{
			Key:   unbondingTxInfo.Key,
			Proof: unbondingTxInfo.Proof,
		},
	}
	_, err = tm.BabylonClient.ReliablySendMsg(context.Background(), msgUndel, nil, nil)
	require.NoError(t, err)
	t.Logf("submitted MsgBTCUndelegate")

	// wait until unbonding tx is on Bitcoin
	require.Eventually(t, func() bool {
		resp, err := tm.WalletClient.GetRawTransactionVerbose(unbondingTxHash)
		if err != nil {
			t.Logf("err of GetRawTransactionVerbose: %v", err)
			return false
		}
		return len(resp.BlockHash) > 0
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return unbondingSlashingInfo, unbondingTxSchnorrSig
}

func getTxInfo(t *testing.T, block *wire.MsgBlock) *btcctypes.TransactionInfo {
	mHeaderBytes := bbn.NewBTCHeaderBytesFromBlockHeader(&block.Header)
	var txBytes [][]byte
	for _, tx := range block.Transactions {
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		_ = tx.Serialize(buf)
		txBytes = append(txBytes, buf.Bytes())
	}
	spvProof, err := btcctypes.SpvProofFromHeaderAndTransactions(&mHeaderBytes, txBytes, 1)
	require.NoError(t, err)
	return btcctypes.NewTransactionInfoFromSpvProof(spvProof)
}

// TODO: these functions should be enabled by Babylon
func bbnPksToBtcPks(pks []bbn.BIP340PubKey) ([]*btcec.PublicKey, error) {
	btcPks := make([]*btcec.PublicKey, 0, len(pks))
	for _, pk := range pks {
		btcPk, err := pk.ToBTCPK()
		if err != nil {
			return nil, err
		}
		btcPks = append(btcPks, btcPk)
	}
	return btcPks, nil
}

func outIdx(tx *wire.MsgTx, candOut *wire.TxOut) (uint32, error) {
	for idx, out := range tx.TxOut {
		if bytes.Equal(out.PkScript, candOut.PkScript) && out.Value == candOut.Value {
			return uint32(idx), nil
		}
	}
	return 0, fmt.Errorf("couldn't find output")
}

func (tm *TestManager) getHighUTXOAndSum() (*btcjson.ListUnspentResult, float64, error) {
	utxos, err := tm.WalletClient.ListUnspent()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list unspent UTXOs: %w", err)
	}
	if len(utxos) == 0 {
		return nil, 0, fmt.Errorf("lack of spendable transactions in the wallet")
	}

	highUTXO := utxos[0] // freshest UTXO
	sum := float64(0)
	for _, utxo := range utxos {
		if highUTXO.Amount < utxo.Amount {
			highUTXO = utxo
		}
		sum += utxo.Amount
	}
	return &highUTXO, sum, nil
}

func (tm *TestManager) SubmitInclusionProof(t *testing.T, stakingTxHash string, txInfo *btcctypes.TransactionInfo) {
	msg := &bstypes.MsgAddBTCDelegationInclusionProof{
		Signer:        tm.MustGetBabylonSigner(),
		StakingTxHash: stakingTxHash,
		StakingTxInclusionProof: &bstypes.InclusionProof{
			Key:   txInfo.Key,
			Proof: txInfo.Proof,
		},
	}

	_, err := tm.BabylonClient.ReliablySendMsg(context.Background(), msg, nil, nil)
	require.NoError(t, err)
}
