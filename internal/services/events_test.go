package services

import (
	"testing"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
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
