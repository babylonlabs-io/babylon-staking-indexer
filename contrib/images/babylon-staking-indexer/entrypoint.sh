#!/usr/bin/env sh
set -euo pipefail
set -x

BINARY=${BINARY:-/bin/babylon-staking-indexer}
CONFIG=${CONFIG:-/home/babylon-staking-indexer/config.yml}

if ! [ -f "${BINARY}" ]; then
    echo "The binary $(basename "${BINARY}") cannot be found."
    exit 1
fi

if ! [ -f "${CONFIG}" ]; then
    echo "The configuration file $(basename "${CONFIG}") cannot be found. Please add the configuration file to the shared folder. Use the CONFIG environment variable if the name of the configuration file is not 'config.yml'"
    exit 1
fi

$BINARY --config "$CONFIG" 2>&1