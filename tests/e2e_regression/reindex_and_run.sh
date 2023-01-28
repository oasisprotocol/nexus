#!/bin/bash

# Wrapper around run.sh that reindexes the database (from scratch to a fixed height)
# before running the e2e regression tests.

set -euo pipefail

cat >/tmp/e2e_config.yaml <<EOF
analysis:
  analyzers:
    - name: emerald_damask
      chain_id: oasis-3
      rpc: unix:/tmp/node.sock
      chaincontext: b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535
      from: 1_003_298  # round at Damask genesis
      to: 1_003_308  # 10 blocks; very fast, for early testing
  storage:
    endpoint: postgresql://rwuser:password@localhost:5432/indexer?sslmode=disable
    backend: postgres
    DANGER__WIPE_STORAGE_ON_STARTUP: true
  migrations: file://storage/migrations

server:
  chain_id: oasis-3
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
./oasis-indexer --config=/tmp/e2e_config.yaml analyze
./oasis-indexer --config=/tmp/e2e_config.yaml serve &
sleep 1
while ! curl --silent localhost:8008/v1/ >/dev/null; do
  echo "Waiting for API server to start..."
  sleep 1
done
tests/e2e_regression/run.sh

