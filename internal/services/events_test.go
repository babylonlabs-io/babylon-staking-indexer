package services

import (
	"testing"

	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"encoding/json"
)

func Test_sanitizeEvent(t *testing.T) {
	cases := []struct {
		name  string
		event string
	}{
		{
			name:  "Moniker value starts with [",
			event: `{"type":"babylon.btcstaking.v1.EventFinalityProviderEdited","attributes":[{"key":"btc_pk_hex","value":"\"fc8a5b9930c3383e94bd940890e93cfcf95b2571ad50df8063b7011f120b918a\"","index":true},{"key":"commission","value":"\"0.030000000000000000\"","index":true},{"key":"details","value":"\"pSTAKE Finance is a multichain liquid staking protocol, backed by Binance Labs.\"","index":true},{"key":"identity","value":"\"CCD58C1559B694A8\"","index":true},{"key":"moniker","value":"\"[Deprecating on 10 Aug 25] pSTAKE Finance\"","index":true},{"key":"security_contact","value":"\"hello@pstake.finance\"","index":true},{"key":"website","value":"\"https://pstake.finance/\"","index":true},{"key":"msg_index","value":"0","index":true}]}`,
		},
	}

	for _, cse := range cases {
		var event abcitypes.Event
		err := json.Unmarshal([]byte(cse.event), &event)
		require.NoError(t, err)

		sanitizedEvent := sanitizeEvent(event)

		// Use the SDK's ParseTypedEvent function
		_, err = sdk.ParseTypedEvent(sanitizedEvent)
		assert.NoError(t, err)
	}
}

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
