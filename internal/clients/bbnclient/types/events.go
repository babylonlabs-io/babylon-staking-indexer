package bbntypes

// Below are temporary types while waiting for core to fix the event type

// EventFinalityProviderCreated is the event emitted when a finality provider is created
type EventFinalityProviderCreated struct {
	// btc_pk is the Bitcoin secp256k1 PK of this finality provider
	// the PK follows encoding in BIP-340 spec
	BtcPk string `protobuf:"bytes,1,opt,name=btc_pk,proto3" json:"btc_pk,omitempty"`
	// addr is the address to receive commission from delegations.
	Addr string `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	// commission defines the commission rate of the finality provider.
	Commission string `protobuf:"bytes,3,opt,name=commission,proto3" json:"commission,omitempty"`
	// description defines the description terms for the finality provider.
	// moniker defines a human-readable name for the validator.
	Moniker string `protobuf:"bytes,1,opt,name=moniker,proto3" json:"moniker,omitempty"`
	// identity defines an optional identity signature (ex. UPort or Keybase).
	Identity string `protobuf:"bytes,2,opt,name=identity,proto3" json:"identity,omitempty"`
	// website defines an optional website link.
	Website string `protobuf:"bytes,3,opt,name=website,proto3" json:"website,omitempty"`
	// security_contact defines an optional email for security contact.
	SecurityContact string `protobuf:"bytes,4,opt,name=security_contact,json=securityContact,proto3" json:"security_contact,omitempty"`
	// details define other optional details.
	Details string `protobuf:"bytes,5,opt,name=details,proto3" json:"details,omitempty"`
}

type EventFinalityProviderStateChange struct {
	// btc_pk is the BTC public key of the finality provider
	BtcPk string `protobuf:"bytes,1,opt,name=btc_pk,json=btcPk,proto3" json:"btc_pk,omitempty"`
	// new_state is the new state that the finality provider
	// is transitioned to
	NewState string `protobuf:"bytes,2,opt,name=new_state,json=newState,proto3" json:"new_state,omitempty"`
}

// EventFinalityProviderEdited is the event emitted when a finality provider is edited
type EventFinalityProviderEdited struct {
	// btc_pk is the Bitcoin secp256k1 PK of this finality provider
	// the PK follows encoding in BIP-340 spec
	BtcPk string `protobuf:"bytes,1,opt,name=btc_pk,proto3" json:"btc_pk,omitempty"`
	// commission defines the commission rate of the finality provider.
	Commission string `protobuf:"bytes,2,opt,name=commission,proto3" json:"commission,omitempty"`
	// moniker defines a human-readable name for the validator.
	Moniker string `protobuf:"bytes,1,opt,name=moniker,proto3" json:"moniker,omitempty"`
	// identity defines an optional identity signature (ex. UPort or Keybase).
	Identity string `protobuf:"bytes,2,opt,name=identity,proto3" json:"identity,omitempty"`
	// website defines an optional website link.
	Website string `protobuf:"bytes,3,opt,name=website,proto3" json:"website,omitempty"`
	// security_contact defines an optional email for security contact.
	SecurityContact string `protobuf:"bytes,4,opt,name=security_contact,json=securityContact,proto3" json:"security_contact,omitempty"`
	// details define other optional details.
	Details string `protobuf:"bytes,5,opt,name=details,proto3" json:"details,omitempty"`
}
