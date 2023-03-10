#!/bin/bash

# Wrapper around run.sh that reindexes the database (from scratch to a fixed height)
# before running the e2e regression tests.

set -euo pipefail

cat >/tmp/e2e_config.yaml <<EOF
analysis:
  node:
    chain_id: oasis-3
    rpc: unix:/tmp/node.sock
    chaincontext: b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535
    fast_startup: true
  analyzers:
    consensus:
      from: 8_048_956  # Damask genesis
      to: 8_049_056  # 100 blocks; fast enough for early testing
    emerald:
      from: 1_003_298  # round at Damask genesis
      to: 1_003_398  # 100 blocks; fast enough for early testing
    evm_tokens: {}
    aggregate_stats:
      tx_volume_interval: 5m
    metadata_registry:
      interval: 5m
  storage:
    endpoint: postgresql://rwuser:password@localhost:5432/indexer?sslmode=disable
    backend: postgres
    DANGER__WIPE_STORAGE_ON_STARTUP: true
    migrations: file://storage/migrations

server:
  chain_id: oasis-3
  chain_name: mainnet
  endpoint: localhost:8008
  storage:
    endpoint: postgresql://api:password@localhost:5432/indexer?sslmode=disable
    backend: postgres

log:
  level: debug
  format: json

metrics:
  pull_endpoint: localhost:8009
EOF

# Kill background processes on exit. (In our case the indexer API server.)
trap 'trap - SIGTERM && kill -- -$$' SIGINT SIGTERM EXIT

make oasis-indexer
./oasis-indexer --config=/tmp/e2e_config.yaml analyze | tee /tmp/analyze.out &
analyzer_pid=$!

# Count how many block analyzers are enabled in the config.
n_block_analyzers=0
for analyzer in consensus emerald sapphire cipher; do
  if grep -qE "^ *${analyzer}:" /tmp/e2e_config.yaml; then
    n_block_analyzers=$((n_block_analyzers + 1))
  fi
done

# Wait for blocks analyzers to be done, then kill the entire indexer.
# It won't terminate on its own because the evm_tokens analyzer is always looking for more work.
while (( $(grep --count "finished processing all blocks" /tmp/analyze.out) < n_block_analyzers )); do
  echo "Waiting for $n_block_analyzers block analyzers to finish..."
  sleep 1
done
sleep 2  # Give evm_tokens analyzer (and other non-block analyzers) a chance to finish.
kill $analyzer_pid

./oasis-indexer --config=/tmp/e2e_config.yaml serve &
while ! curl --silent localhost:8008/v1/ >/dev/null; do
  echo "Waiting for API server to start..."
  sleep 1
done
tests/e2e_regression/run.sh

