#!/bin/bash

# Wrapper around run.sh that reindexes the database (from scratch to a fixed height)
# before running the e2e regression tests.

set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Kill background processes on exit. (In our case the indexer API server.)
trap 'trap - SIGTERM && kill -- -$$' SIGINT SIGTERM EXIT

make nexus
./nexus --config="${SCRIPT_DIR}/e2e_config.yaml" analyze | tee /tmp/analyze.out &
analyzer_pid=$!

# Count how many block analyzers are enabled in the config.
n_block_analyzers=0
for analyzer in consensus emerald sapphire cipher; do
  if grep -qE "^ *${analyzer}:" "${SCRIPT_DIR}/e2e_config.yaml"; then
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

tests/e2e_regression/run.sh

