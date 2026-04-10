package services

import (
	"testing"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sanitizeEvent_MaliciousValues(t *testing.T) {
	// Base event template: a valid EventFinalityProviderEdited with a placeholder moniker.
	// The moniker value is replaced per test case.
	makeEvent := func(monikerValue string) abcitypes.Event {
		return abcitypes.Event{
			Type: "babylon.btcstaking.v1.EventFinalityProviderEdited",
			Attributes: []abcitypes.EventAttribute{
				{Key: "btc_pk_hex", Value: `"fc8a5b9930c3383e94bd940890e93cfcf95b2571ad50df8063b7011f120b918a"`},
				{Key: "commission", Value: `"0.030000000000000000"`},
				{Key: "details", Value: `"some details"`},
				{Key: "identity", Value: `""`},
				{Key: "moniker", Value: monikerValue},
				{Key: "security_contact", Value: `""`},
				{Key: "website", Value: `""`},
				{Key: "msg_index", Value: "0"},
			},
		}
	}

	tests := []struct {
		name         string
		monikerValue string
	}{
		{
			name:         "normal moniker",
			monikerValue: `"my-validator"`,
		},
		{
			name:         "moniker starting with { (not valid JSON)",
			monikerValue: `"{evil_moniker"`,
		},
		{
			name:         "moniker starting with [ (not valid JSON)",
			monikerValue: `"[not an array"`,
		},
		{
			name:         "moniker that is only {",
			monikerValue: `"{"`,
		},
		{
			name:         "moniker with curly braces inside",
			monikerValue: `"{hello}{world}"`,
		},
		{
			name:         "moniker starting with [ and ending with ]  but not JSON",
			monikerValue: `"[Deprecating on 10 Aug 25] pSTAKE Finance"`,
		},
		{
			// A moniker that happens to be valid JSON is still a string field in
			// protobuf, so it must be quoted. The chain delivers it quoted.
			name:         "moniker that looks like JSON but is a string field",
			monikerValue: `"{\"key\":\"value\"}"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := makeEvent(tt.monikerValue)
			sanitized := sanitizeEvent(event)

			// The sanitized event must always be parseable by ParseTypedEvent.
			// Before the fix, monikers starting with '{' or '[' that aren't valid
			// JSON would be left unquoted, causing ParseTypedEvent to fail.
			_, err := sdk.ParseTypedEvent(sanitized)
			require.NoError(t, err, "ParseTypedEvent should not fail on sanitized event")

			// Verify the moniker attribute is valid JSON after sanitization
			for _, attr := range sanitized.Attributes {
				if attr.Key == "moniker" {
					assert.True(t,
						(attr.Value[0] == '"') || (attr.Value[0] == '{') || (attr.Value[0] == '['),
						"moniker value should be a valid JSON token, got: %s", attr.Value,
					)
				}
			}
		})
	}
}
