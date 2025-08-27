#!/bin/bash
set -euo pipefail

HEIGHT="1698618"
ARCHIVE_RPC="https://babylon-testnet-rpc-archive-1.nodes.guru"
URL="${ARCHIVE_RPC}/block_results?height=${HEIGHT}"
OUTPUT_FILE="block_${HEIGHT}_allowlist_results.json"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl not found" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq not found" >&2
  exit 1
fi

echo "# Fetching block_results for MVP allow-list parsing at height=${HEIGHT}"
echo "# Output will be saved to: ${OUTPUT_FILE}"

JSON=$(curl -sSL "$URL")

# Extract and structure allow-list data for MVP with raw block results
printf "%s" "$JSON" | jq '{
  block_height: .result.height,
  timestamp: now,
  events: (
    (
      (.result?.txs_results // [] | map(.events) | add // []) +
      (.result?.finalize_block_events // [])
    )
    | map(select(.type=="instantiate" or .type=="wasm"))
    | map({
        type: .type,
        attributes: (.attributes | map({key: .key, value: .value}) | from_entries),
        contract_address: (.attributes[] | select(.key=="_contract_address") | .value),
        allow_list: (.attributes[] | select(.key=="allow-list") | .value),
        action: (.attributes[] | select(.key=="action") | .value)
      })
    | map(select(.contract_address != null or .allow_list != null))
  ),
  summary: {
    total_events: (
      (
        (.result?.txs_results // [] | map(.events) | add // []) +
        (.result?.finalize_block_events // [])
      )
      | map(select(.type=="instantiate" or .type=="wasm")) | length
    ),
    contracts_with_allowlist: [
      (
        (
          (.result?.txs_results // [] | map(.events) | add // []) +
          (.result?.finalize_block_events // [])
        )
        | map(select(.type=="instantiate" or .type=="wasm"))
        | map(select(.attributes[]?.key=="allow-list"))
        | map(.attributes[] | select(.key=="_contract_address") | .value)
        | unique
      )[]
    ]
  },
  raw_block_results: .
}' > "$OUTPUT_FILE"

echo "# Results exported to: $OUTPUT_FILE"
echo "# Summary:"
jq -r '.summary | "Total events: \(.total_events)\nContracts with allow-list: \(.contracts_with_allowlist | join(", "))"' "$OUTPUT_FILE"

# Also show human-readable format
echo -e "\n# Human-readable events with allow-list focus:"
printf "%s" "$JSON" | jq -r '
  (
    (.result?.txs_results // [] | map(.events) | add // []) +
    (.result?.finalize_block_events // [])
  )
  | map(select(.type=="instantiate" or .type=="wasm"))
  | .[]
  | "type=" + .type + "\n" + ([.attributes[]? | (.key + "=" + .value)] | join("\n")) + "\n---"' | sed -E 's/(^|\n)(allow-list=)/\1>>> \2/g'
