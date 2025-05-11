//go:build integration

package db_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/db/model"
	"github.com/babylonlabs-io/babylon-staking-indexer/testutil"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalityProvider(t *testing.T) {
	ctx := t.Context()
	t.Cleanup(func() {
		resetDatabase(t)
	})

	t.Run("save", func(t *testing.T) {
		fp := &model.FinalityProviderDetails{
			BtcPk:       randomBTCpk(t),
			State:       bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE.String(),
			Description: model.Description{},
		}
		err := testDB.SaveNewFinalityProvider(ctx, fp)
		require.NoError(t, err)

		// saving fp with the same BtcPk must trigger an error
		err = testDB.SaveNewFinalityProvider(ctx, &model.FinalityProviderDetails{
			BtcPk: fp.BtcPk,
		})
		require.Error(t, err)
		assert.True(t, db.IsDuplicateKeyError(err))

		// passing nil as finality provider should return original mongo error
		err = testDB.SaveNewFinalityProvider(ctx, nil)
		assert.Error(t, err)
	})
	t.Run("get", func(t *testing.T) {
		fp, err := testDB.GetFinalityProviderByBtcPk(ctx, "non-existent")
		require.Error(t, err)
		assert.True(t, db.IsNotFoundError(err))
		assert.Nil(t, fp)

		fp = &model.FinalityProviderDetails{
			BtcPk: randomBTCpk(t),
			State: bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE.String(),
		}
		err = testDB.SaveNewFinalityProvider(ctx, fp)
		require.NoError(t, err)

		foundFP, err := testDB.GetFinalityProviderByBtcPk(ctx, fp.BtcPk)
		require.NoError(t, err)
		assert.Equal(t, fp, foundFP)
	})
	t.Run("update", func(t *testing.T) {
		t.Run("state", func(t *testing.T) {
			// first check non-existing finality provider
			newState := bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE
			err := testDB.UpdateFinalityProviderState(ctx, "non-existent", newState.String())
			require.Error(t, err)
			assert.True(t, db.IsNotFoundError(err))

			// check main case: update of existing finality provider
			fp := &model.FinalityProviderDetails{
				BtcPk: randomBTCpk(t),
				State: bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE.String(),
			}
			// fp.state above must be different from newState
			// otherwise the test won't make sense - we won't be able to check if the state was updated
			assert.NotEqual(t, newState.String(), fp.State)
			err = testDB.SaveNewFinalityProvider(ctx, fp)
			require.NoError(t, err)

			err = testDB.UpdateFinalityProviderState(ctx, fp.BtcPk, newState.String())
			require.NoError(t, err)

			foundFP, err := testDB.GetFinalityProviderByBtcPk(ctx, fp.BtcPk)
			require.NoError(t, err)
			assert.Equal(t, newState.String(), foundFP.State)
		})
		t.Run("details", func(t *testing.T) {
			// no fields to update - no error
			err := testDB.UpdateFinalityProviderDetailsFromEvent(ctx, &model.FinalityProviderDetails{})
			assert.NoError(t, err)

			// if there are fields to update, but record doesn't exist - there should be not found error
			err = testDB.UpdateFinalityProviderDetailsFromEvent(ctx, &model.FinalityProviderDetails{
				Commission: "0.1",
			})
			require.Error(t, err)
			assert.True(t, db.IsNotFoundError(err))

			// main case - update existing finality provider
			fp := &model.FinalityProviderDetails{
				BtcPk:          randomBTCpk(t),
				BabylonAddress: "original-babylon-address",
				Commission:     "0.1",
				State:          bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE.String(),
				Description: model.Description{
					Moniker:         "moniker0",
					Identity:        "identity0",
					Website:         "website0",
					SecurityContact: "security_contact0",
					Details:         "security_details0",
				},
			}
			err = testDB.SaveNewFinalityProvider(ctx, fp)
			require.NoError(t, err)

			fpUpdate := &model.FinalityProviderDetails{
				BtcPk:          fp.BtcPk,
				BabylonAddress: "babylon-address",
				Commission:     "0.5",
				State:          bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_ACTIVE.String(),
				Description: model.Description{
					Moniker:         "moniker1",
					Identity:        "identity1",
					Website:         "website1",
					SecurityContact: "security_contact1",
					Details:         "security_details1",
				},
			}
			err = testDB.UpdateFinalityProviderDetailsFromEvent(ctx, fpUpdate)
			require.NoError(t, err)

			foundFP, err := testDB.GetFinalityProviderByBtcPk(ctx, fp.BtcPk)
			require.NoError(t, err)
			// first check fields that should not be updated
			assert.NotEqual(t, fpUpdate.BabylonAddress, foundFP.BabylonAddress)
			assert.NotEqual(t, fpUpdate.State, foundFP.State)
			// now check fields that should be updated
			assert.Equal(t, fpUpdate.Commission, foundFP.Commission)
			assert.Equal(t, fpUpdate.Description, foundFP.Description)
		})
	})
}

func randomBTCpk(t *testing.T) string {
	result, err := testutil.RandomAlphaNum(10)
	require.NoError(t, err)

	return result
}
