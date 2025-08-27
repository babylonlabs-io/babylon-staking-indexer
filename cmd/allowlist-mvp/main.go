package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/clients/bbnclient"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/config"
	"github.com/babylonlabs-io/babylon-staking-indexer/internal/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
)

// AllowlistMVPResult represents the MVP output structure for multiple blocks
type AllowlistMVPResult struct {
	Timestamp    int64               `json:"timestamp"`
	BlockResults []BlockResult       `json:"block_results"`
	TotalSummary AllowlistSummary    `json:"total_summary"`
}

// BlockResult represents the result for a single block
type BlockResult struct {
	BlockHeight     int64                  `json:"block_height"`
	AllowlistEvents []types.AllowlistEvent `json:"allowlist_events"`
	Summary         AllowlistSummary       `json:"summary"`
}

// RawBlockResult represents raw blockchain data for a single block
type RawBlockResult struct {
	BlockHeight     int64       `json:"block_height"`
	RawBlockResults interface{} `json:"raw_block_results"`
}

// RawResultsFile represents the structure for the raw results file
type RawResultsFile struct {
	Timestamp       int64             `json:"timestamp"`
	RawBlockResults []RawBlockResult  `json:"raw_block_results"`
}

type AllowlistSummary struct {
	TotalEvents               int      `json:"total_events"`
	ContractsWithAllowlist    []string `json:"contracts_with_allowlist"`
	InstantiateEvents         int      `json:"instantiate_events"`
	AddToAllowlistEvents      int      `json:"add_to_allowlist_events"`
	RemoveFromAllowlistEvents int      `json:"remove_from_allowlist_events"`
}

func main() {

	// Configuration for testnet archive
	cfg := &config.BBNConfig{
		RPCAddr:       "https://babylon-testnet-rpc-archive-1.nodes.guru",
		Timeout:       30 * time.Second,
		MaxRetryTimes: 3,
		RetryInterval: 1 * time.Second,
	}

	// Target block heights for MVP (can add more blocks to analyze)
	blockHeights := []int64{
		1698618, // Original testnet instantiate example
		1824230, // Remove allowlist event block
		1824231, // Add allowlist event block
	}

	// Create BBN client using existing infrastructure
	client, err := bbnclient.NewBBNClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create BBN client: %v", err)
	}

	ctx := context.Background()

	fmt.Printf("# Fetching block_results for allowlist MVP at heights: %v\n", blockHeights)
	fmt.Printf("# Using existing indexer infrastructure\n")

	// Process each block height
	var blockResults []BlockResult
	var allRawResults []RawBlockResult
	var allEvents []types.AllowlistEvent
	var allContracts = make(map[string]bool)

	for _, blockHeight := range blockHeights {
		fmt.Printf("\n## Processing block height: %d\n", blockHeight)
		
		// Fetch block results using existing client
		rawBlockResults, err := client.GetBlockResults(ctx, &blockHeight)
		if err != nil {
			log.Printf("Failed to get block results for height %d: %v", blockHeight, err)
			continue
		}

		// Parse allowlist events
		allowlistEvents, err := parseAllowlistEventsFromBlockResults(rawBlockResults)
		if err != nil {
			log.Printf("Failed to parse allowlist events for height %d: %v", blockHeight, err)
			continue
		}

		// Create summary for this block
		summary := createAllowlistSummary(allowlistEvents)

		// Add to block results
		blockResult := BlockResult{
			BlockHeight:     blockHeight,
			AllowlistEvents: allowlistEvents,
			Summary:         summary,
		}
		blockResults = append(blockResults, blockResult)

		// Store raw block results separately
		rawResult := RawBlockResult{
			BlockHeight:     blockHeight,
			RawBlockResults: rawBlockResults,
		}
		allRawResults = append(allRawResults, rawResult)

		// Aggregate data for total summary
		allEvents = append(allEvents, allowlistEvents...)
		for _, contract := range summary.ContractsWithAllowlist {
			allContracts[contract] = true
		}

		fmt.Printf("Block %d: Found %d allowlist events\n", blockHeight, len(allowlistEvents))
	}

	// Create total summary
	totalSummary := createAllowlistSummary(allEvents)

	// Create final result structure (without raw data)
	result := AllowlistMVPResult{
		Timestamp:    time.Now().Unix(),
		BlockResults: blockResults,
		TotalSummary: totalSummary,
	}

	// Create raw results structure
	rawResults := RawResultsFile{
		Timestamp:       time.Now().Unix(),
		RawBlockResults: allRawResults,
	}

	// Output main results to JSON file
	outputFile := fmt.Sprintf("babylon-staking-indexer/allowlist_mvp_blocks_%s.json", formatBlockHeights(blockHeights))
	if err := saveAllowlistResultToFile(result, outputFile); err != nil {
		log.Fatalf("Failed to save result: %v", err)
	}

	// Output raw results to separate JSON file
	rawOutputFile := fmt.Sprintf("babylon-staking-indexer/allowlist_mvp_raw_blocks_%s.json", formatBlockHeights(blockHeights))
	if err := saveRawResultToFile(rawResults, rawOutputFile); err != nil {
		log.Fatalf("Failed to save raw results: %v", err)
	}

	// Print summary
	fmt.Printf("\n# Results exported to: %s\n", outputFile)
	fmt.Printf("# Raw block data exported to: %s\n", rawOutputFile)
	fmt.Printf("# Total Summary across all blocks:\n")
	fmt.Printf("Total blocks processed: %d\n", len(blockResults))
	fmt.Printf("Total allowlist events: %d\n", totalSummary.TotalEvents)
	fmt.Printf("Instantiate events: %d\n", totalSummary.InstantiateEvents)
	fmt.Printf("Add to allowlist events: %d\n", totalSummary.AddToAllowlistEvents)
	fmt.Printf("Remove from allowlist events: %d\n", totalSummary.RemoveFromAllowlistEvents)
	fmt.Printf("Contracts with allowlist: %v\n", totalSummary.ContractsWithAllowlist)

	// Print detailed summary for each block
	fmt.Printf("\n# Per-block Summary:\n")
	for _, blockResult := range blockResults {
		fmt.Printf("Block %d: %d events (%d instantiate, %d add, %d remove)\n",
			blockResult.BlockHeight,
			blockResult.Summary.TotalEvents,
			blockResult.Summary.InstantiateEvents,
			blockResult.Summary.AddToAllowlistEvents,
			blockResult.Summary.RemoveFromAllowlistEvents)
	}

	// Print detailed events for all blocks
	fmt.Printf("\n# Detailed Allowlist Events:\n")
	eventCount := 1
	for _, blockResult := range blockResults {
		if len(blockResult.AllowlistEvents) > 0 {
			fmt.Printf("\n### Block %d Events:\n", blockResult.BlockHeight)
			for _, event := range blockResult.AllowlistEvents {
				fmt.Printf("Event %d:\n", eventCount)
				fmt.Printf("  Block: %d\n", blockResult.BlockHeight)
				fmt.Printf("  Type: %s\n", event.EventType)
				fmt.Printf("  Contract: %s\n", event.ContractAddress)
				fmt.Printf("  Action: %s\n", event.Action)

				pubkeys := event.GetPubkeys()
				if len(pubkeys) > 0 {
					fmt.Printf("  Pubkeys (%d): %s\n", len(pubkeys),
						truncateStringSlice(pubkeys, 3))
				}
				fmt.Println()
				eventCount++
			}
		}
	}
}

// parseAllowlistEventsFromBlockResults extracts allowlist events from block results
func parseAllowlistEventsFromBlockResults(blockResults interface{}) ([]types.AllowlistEvent, error) {
	// Always use JSON parsing since the BBN client returns a complex structure
	return parseAllowlistEventsFromJSON(blockResults)
}

// parseAllowlistEventsFromJSON is a fallback parser for JSON data
func parseAllowlistEventsFromJSON(blockResults interface{}) ([]types.AllowlistEvent, error) {
	var events []types.AllowlistEvent

	// Convert to JSON and back to extract events
	jsonData, err := json.Marshal(blockResults)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block results: %w", err)
	}

	var rawResult map[string]interface{}
	if err := json.Unmarshal(jsonData, &rawResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block results: %w", err)
	}

	// The BBN client returns the block results directly, not wrapped in a "result" field
	// So we work with rawResult directly
	result := rawResult

	// Process transaction events
	if txsResults, ok := result["txs_results"].([]interface{}); ok {
		for _, txResult := range txsResults {
			if txMap, ok := txResult.(map[string]interface{}); ok {
				if eventsInterface, ok := txMap["events"].([]interface{}); ok {
					for _, eventInterface := range eventsInterface {
						if eventMap, ok := eventInterface.(map[string]interface{}); ok {
							event := mapToABCIEvent(eventMap)
							if types.IsAllowlistEvent(types.EventType(event.Type)) {
								allowlistEvent, err := types.ParseAllowlistEvent(event)
								if err != nil {
									continue
								}
								events = append(events, *allowlistEvent)
							}
						}
					}
				}
			}
		}
	}

	// Process finalize block events
	if finalizeEvents, ok := result["finalize_block_events"].([]interface{}); ok {
		for _, eventInterface := range finalizeEvents {
			if eventMap, ok := eventInterface.(map[string]interface{}); ok {
				event := mapToABCIEvent(eventMap)
				if types.IsAllowlistEvent(types.EventType(event.Type)) {
					allowlistEvent, err := types.ParseAllowlistEvent(event)
					if err != nil {
						continue
					}
					events = append(events, *allowlistEvent)
				}
			}
		}
	}

	return events, nil
}

// mapToABCIEvent converts a map to an ABCI event
func mapToABCIEvent(eventMap map[string]interface{}) abcitypes.Event {
	event := abcitypes.Event{}

	if eventType, ok := eventMap["type"].(string); ok {
		event.Type = eventType
	}

	if attributesInterface, ok := eventMap["attributes"].([]interface{}); ok {
		for _, attrInterface := range attributesInterface {
			if attrMap, ok := attrInterface.(map[string]interface{}); ok {
				attr := abcitypes.EventAttribute{}
				if key, ok := attrMap["key"].(string); ok {
					attr.Key = key
				}
				if value, ok := attrMap["value"].(string); ok {
					attr.Value = value
				}
				event.Attributes = append(event.Attributes, attr)
			}
		}
	}

	return event
}

// createAllowlistSummary creates a summary of allowlist events
func createAllowlistSummary(events []types.AllowlistEvent) AllowlistSummary {
	summary := AllowlistSummary{
		TotalEvents: len(events),
	}

	contractsMap := make(map[string]bool)

	for _, event := range events {
		// Track contracts with allowlists
		if len(event.GetPubkeys()) > 0 {
			contractsMap[event.ContractAddress] = true
		}

		// Count event types
		if event.IsInstantiateEvent() {
			summary.InstantiateEvents++
		} else if event.IsAddEvent() {
			summary.AddToAllowlistEvents++
		} else if event.IsRemoveEvent() {
			summary.RemoveFromAllowlistEvents++
		}
	}

	// Convert map to slice
	for contract := range contractsMap {
		summary.ContractsWithAllowlist = append(summary.ContractsWithAllowlist, contract)
	}

	return summary
}

// formatBlockHeights creates a string representation of block heights for filename
func formatBlockHeights(heights []int64) string {
	if len(heights) == 1 {
		return fmt.Sprintf("%d", heights[0])
	}
	if len(heights) <= 3 {
		var parts []string
		for _, h := range heights {
			parts = append(parts, fmt.Sprintf("%d", h))
		}
		return fmt.Sprintf("%s", strings.Join(parts, "_"))
	}
	return fmt.Sprintf("%d_to_%d_plus%d", heights[0], heights[len(heights)-1], len(heights)-2)
}

// saveAllowlistResultToFile saves the allowlist result to a JSON file
func saveAllowlistResultToFile(result AllowlistMVPResult, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// saveRawResultToFile saves the raw blockchain data to a separate JSON file
func saveRawResultToFile(rawResults RawResultsFile, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(rawResults); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// truncateStringSlice returns a truncated representation of a string slice
func truncateStringSlice(slice []string, maxShow int) string {
	if len(slice) <= maxShow {
		return fmt.Sprintf("%v", slice)
	}

	truncated := make([]string, maxShow)
	copy(truncated, slice[:maxShow])

	return fmt.Sprintf("%v... (+%d more)", truncated, len(slice)-maxShow)
}
