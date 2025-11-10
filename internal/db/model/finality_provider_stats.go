package model

// FinalityProviderStatsDocument represents stats for a finality provider
// Stored in a separate collection from FinalityProviderDetails for safety
type FinalityProviderStatsDocument struct {
	FpBtcPkHex        string `bson:"_id"`                // Primary key - FP BTC public key (lowercase)
	ActiveTvl         uint64 `bson:"active_tvl"`         // Active TVL for this FP in satoshis
	ActiveDelegations uint64 `bson:"active_delegations"` // Active delegation count for this FP
	LastUpdated       int64  `bson:"last_updated"`       // Unix timestamp of last stats update
}
