## Usage

### dump-events
How to run: `go run cmd/babylon-staking-indexer/main.go dump-events --config <config-file>`
You can change `config/config-local.yml`, specifically only part that corresponds to bbn, other keys are not used.
It's better to set values for `maxretrytimes` and `retryinterval` (you can use values from the config file).

Keep in mind that this tool scans blockchain backwards (starting from highest block till the first one).