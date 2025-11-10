package model

// OverallStatsDocument represents the overall staking statistics
type OverallStatsDocument struct {
	ID                string `bson:"_id"`                // Always "overall_stats"
	ActiveTvl         uint64 `bson:"active_tvl"`         // Total active TVL in satoshis
	ActiveDelegations uint64 `bson:"active_delegations"` // Total active delegation count
	LastUpdated       int64  `bson:"last_updated"`       // Unix timestamp of last update
}
