package bbntypes

import "github.com/cometbft/cometbft/abci/types"

// Re-exporting types.Event from cometbft package
type Event = types.Event

type CometBFTRPCResponse[T any] struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  T      `json:"result"`
}

type TransactionResult struct {
	Events []types.Event `json:"events"`
}

type BlockResultsResponse struct {
	Height              string              `json:"height"`
	TxResults           []TransactionResult `json:"tx_results,omitempty"`
	FinalizeBlockEvents []types.Event       `json:"finalize_block_events,omitempty"`
}
