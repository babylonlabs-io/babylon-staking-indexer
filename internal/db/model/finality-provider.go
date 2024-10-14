package model

import (
	types "github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient/types"
	bbntypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

type FinalityProviderDetails struct {
	BtcPk          string      `bson:"_id"` // Primary key
	BabylonAddress string      `bson:"babylon_address"`
	Commission     string      `bson:"commission"`
	State          string      `bson:"state"`
	Description    Description `bson:"description"`
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
	event *types.EventFinalityProviderCreated,
) *FinalityProviderDetails {
	return &FinalityProviderDetails{
		BtcPk:          event.BtcPk,
		BabylonAddress: event.Addr,
		Description: Description{
			Moniker:         event.Moniker,
			Identity:        event.Identity,
			Website:         event.Website,
			SecurityContact: event.SecurityContact,
			Details:         event.Details,
		},
		Commission: event.Commission,
		// TODO: Below to be updated once BBN used string type for state
		State: bbntypes.FinalityProviderStatus_name[int32(bbntypes.FinalityProviderStatus_STATUS_INACTIVE)],
	}
}

func FromEventFinalityProviderEdited(
	event *types.EventFinalityProviderEdited,
) *FinalityProviderDetails {
	return &FinalityProviderDetails{
		BtcPk: event.BtcPk,
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
