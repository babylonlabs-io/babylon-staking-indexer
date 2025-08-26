package model

import (
	"time"

	bbntypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

type FinalityProviderDetails struct {
	BtcPk          string      `bson:"_id"` // Primary key
	BabylonAddress string      `bson:"babylon_address"`
	Commission     string      `bson:"commission"`
	State          string      `bson:"state"`
	Description    Description `bson:"description"`
	BsnID          string      `bson:"bsn_id"`
	Logo           Logo        `bson:"logo,omitempty"`
}

type Logo struct {
	URL           string    `bson:"url"`
	LastUpdatedAt time.Time `bson:"last_updated_at"`
}

// Description represents the nested description field
type Description struct {
	Moniker         string `bson:"moniker"`
	Identity        string `bson:"identity"`
	Website         string `bson:"website"`
	SecurityContact string `bson:"security_contact"`
	Details         string `bson:"details"`
}

func FromEventFinalityProviderCreated(
	event *bbntypes.EventFinalityProviderCreated,
) *FinalityProviderDetails {
	return &FinalityProviderDetails{
		BtcPk:          event.BtcPkHex,
		BabylonAddress: event.Addr,
		Description: Description{
			Moniker:         event.Moniker,
			Identity:        event.Identity,
			Website:         event.Website,
			SecurityContact: event.SecurityContact,
			Details:         event.Details,
		},
		BsnID:      event.BsnId,
		Commission: event.Commission,
		State:      bbntypes.FinalityProviderStatus_FINALITY_PROVIDER_STATUS_INACTIVE.String(),
	}
}

func FromEventFinalityProviderEdited(
	event *bbntypes.EventFinalityProviderEdited,
) *FinalityProviderDetails {
	return &FinalityProviderDetails{
		BtcPk: event.BtcPkHex,
		Description: Description{
			Moniker:         event.Moniker,
			Identity:        event.Identity,
			Website:         event.Website,
			SecurityContact: event.SecurityContact,
			Details:         event.Details,
		},
		Commission: event.Commission,
	}
}
