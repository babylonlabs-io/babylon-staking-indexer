//go:build integration

package db_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegation(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	t.Run("get", func(t *testing.T) {
		t.Run("by staking tx hash", func(t *testing.T) {
			// there are other tests that cover happy path, here we just check correct error is returned
			details, err := testDB.GetBTCDelegationByStakingTxHash(ctx, randomStakingTxHashHex(t))
			require.Error(t, err)
			assert.True(t, db.IsNotFoundError(err))
			assert.Nil(t, details)
		})
		t.Run("by finality provider", func(t *testing.T) {
			delegation := createDelegation(t)
			err := testDB.SaveNewBTCDelegation(ctx, delegation)
			require.NoError(t, err)
			// just in case gofake don't fill this field (we need at least 1)
			require.NotEmpty(t, delegation.FinalityProviderBtcPksHex)

			items, err := testDB.GetDelegationsByFinalityProvider(ctx, delegation.FinalityProviderBtcPksHex[0])
			require.NoError(t, err)
			assert.Contains(t, items, delegation)
		})
		t.Run("by states", func(t *testing.T) {
			delegation := createDelegation(t)
			delegation.State = types.StatePending
			err := testDB.SaveNewBTCDelegation(ctx, delegation)
			require.NoError(t, err)

			// first check that there no records has been returned
			items, err := testDB.GetBTCDelegationsByStates(ctx, []types.DelegationState{types.StateActive})
			require.NoError(t, err)
			assert.Empty(t, items)

			// now do the same, but this time use state of delegation
			items, err = testDB.GetBTCDelegationsByStates(ctx, []types.DelegationState{delegation.State})
			require.NoError(t, err)
			require.Len(t, items, 1)
			assert.Contains(t, items, delegation)
		})
	})
	t.Run("save", func(t *testing.T) {
		// error due to nil delegation doc
		err := testDB.SaveNewBTCDelegation(ctx, nil)
		require.Error(t, err)

		// successful save
		delegation := createDelegation(t)
		err = testDB.SaveNewBTCDelegation(ctx, delegation)
		require.NoError(t, err)

		state, err := testDB.GetBTCDelegationState(ctx, delegation.StakingTxHashHex)
		require.NoError(t, err)
		assert.Equal(t, &delegation.State, state)

		// error due to duplicate key
		delegation2 := createDelegation(t)
		delegation2.StakingTxHashHex = delegation.StakingTxHashHex
		err = testDB.SaveNewBTCDelegation(ctx, delegation2)
		require.Error(t, err)
		assert.True(t, db.IsDuplicateKeyError(err))

		t.Run("slashing_tx", func(t *testing.T) {
			// first check not found error (empty params are good enough)
			err := testDB.SaveBTCDelegationSlashingTxHex(ctx, "", "", 0)
			require.Error(t, err)
			assert.True(t, db.IsNotFoundError(err))

			delegation := createDelegation(t)
			// zero slashing tx just in case
			delegation.SlashingTx = model.SlashingTx{}
			err = testDB.SaveNewBTCDelegation(ctx, delegation)
			require.NoError(t, err)

			var (
				slashingTxHex  = "slashing_tx_hex"
				spendingHeight = uint32(1)
			)
			err = testDB.SaveBTCDelegationSlashingTxHex(ctx, delegation.StakingTxHashHex, slashingTxHex, spendingHeight)
			require.NoError(t, err)

			item, err := testDB.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
			require.NoError(t, err)

			delegation.SlashingTx = model.SlashingTx{
				SlashingTxHex:  slashingTxHex,
				SpendingHeight: spendingHeight,
			}
			assert.Equal(t, delegation, item)
		})
		t.Run("unbonding slashing_tx", func(t *testing.T) {
			// first check not found error (empty params are good enough)
			err := testDB.SaveBTCDelegationUnbondingSlashingTxHex(ctx, "", "", 0)
			require.Error(t, err)
			assert.True(t, db.IsNotFoundError(err))

			delegation := createDelegation(t)
			// zero slashing tx just in case
			delegation.SlashingTx = model.SlashingTx{}
			err = testDB.SaveNewBTCDelegation(ctx, delegation)
			require.NoError(t, err)

			var (
				unbondingSlashingTxHex = "slashing_tx_hex"
				spendingHeight         = uint32(1)
			)
			err = testDB.SaveBTCDelegationUnbondingSlashingTxHex(ctx, delegation.StakingTxHashHex, unbondingSlashingTxHex, spendingHeight)
			require.NoError(t, err)

			item, err := testDB.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
			require.NoError(t, err)

			delegation.SlashingTx = model.SlashingTx{
				UnbondingSlashingTxHex: unbondingSlashingTxHex,
				SpendingHeight:         spendingHeight,
			}
			assert.Equal(t, delegation, item)
		})
	})
	t.Run("update covenant signatures", func(t *testing.T) {
		delegation := createDelegation(t)
		// by default gofake will fulfill signatures, in this test we don't need it
		delegation.CovenantSignatures = []model.CovenantSignature{}

		err := testDB.SaveNewBTCDelegation(ctx, delegation)
		require.NoError(t, err)

		signatures := []model.CovenantSignature{
			{SignatureHex: "signature_hex_1", CovenantBtcPkHex: "covenant_btc_pk_hex_1"},
			{SignatureHex: "signature_hex_2", CovenantBtcPkHex: "covenant_btc_pk_hex_2", StakeExpansionSignatureHex: "some_stake_expansion_signature_hex"},
		}
		// idea is to update (push) signatures one by one and compare them with expected result (append to delegation struct)
		for i, sig := range signatures {
			err = testDB.SaveBTCDelegationCovenantSignature(ctx, delegation.StakingTxHashHex, sig.CovenantBtcPkHex, sig.SignatureHex, sig.StakeExpansionSignatureHex)
			require.NoError(t, err)

			details, err := testDB.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
			require.NoError(t, err)

			// on every iteration we expect to receive from db only already seen signatures
			delegation.CovenantSignatures = signatures[:i+1]
			assert.Equal(t, delegation, details)
		}
	})
	t.Run("update state", func(t *testing.T) {
		// empty qualified previous states
		err := testDB.UpdateBTCDelegationState(ctx, "non-existent-staking-tx-hash", nil, types.StateActive)
		assert.Error(t, err)

		// no records found
		qualifiedStates := []types.DelegationState{types.StatePending}
		err = testDB.UpdateBTCDelegationState(ctx, "non-existent-staking-tx-hash", qualifiedStates, types.StateActive)
		require.Error(t, err)
		assert.True(t, db.IsNotFoundError(err))
	})
	t.Run("can expand", func(t *testing.T) {
		t.Run("delegation not found", func(t *testing.T) {
			err := testDB.SetBTCDelegationCanExpand(ctx, "b4ffb9d0715be3ffe8bbf11c6ee2e3a49931f141ca6c432f8f3d404f67b79ee8")
			require.True(t, db.IsNotFoundError(err))
		})
		t.Run("ok", func(t *testing.T) {
			delegation := createDelegation(t)
			err := testDB.SaveNewBTCDelegation(ctx, delegation)
			require.NoError(t, err)

			err = testDB.SetBTCDelegationCanExpand(ctx, delegation.StakingTxHashHex)
			require.NoError(t, err)

			foundDelegation, err := testDB.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
			require.NoError(t, err)
			assert.True(t, foundDelegation.CanExpand)
		})
	})
}

func createDelegation(t *testing.T) *model.BTCDelegationDetails {
	var delegation model.BTCDelegationDetails
	err := gofakeit.Struct(&delegation)
	require.NoError(t, err)

	// these fields sometimes cause trouble during save
	// that's why we reset them to zero values (in future we might consider to add fake tags in struct definition)
	delegation.StakingAmount = 0
	delegation.StakingBTCTimestamp = 0
	delegation.UnbondingBTCTimestamp = 0

	return &delegation
}
