package pkg

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ValidateBabylonAddress(address string) error {
	const babylonPrefix = "bbn"
	bz, err := sdk.GetFromBech32(address, babylonPrefix)
	if err != nil {
		return err
	}

	return sdk.VerifyAddressFormat(bz)
}
