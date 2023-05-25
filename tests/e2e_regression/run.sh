#!/bin/bash

# This script is a simple e2e regression test for the indexer.
#
# It fetches a fixed set of URLs from the indexer and saves the responses to files,
# then check that the responses match expected outputs (from a previous run).
#
# If the differences are expected, smiply check the new responses into git.
#
# NOTE: It is the responsibility of the caller to invoke this script against an
# indexer that is SYNCED TO THE SAME HEIGHT as in the run that produced the
# expected outputs.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# The hostname of the indexer to test
hostname="http://localhost:8008"

# The directory to store the actual responses in
outDir="$SCRIPT_DIR/actual"
mkdir -p "$outDir"

testCases=(
  'status                   /v1/'
  'spec                     /v1/spec/v1.yaml'
  'trailing_slash           /v1/consensus/accounts/?limit=1'
  'accounts                 /v1/consensus/accounts'
  'min_balance              /v1/consensus/accounts?minTotalBalance=1000000'
  'big_int_balance          /v1/consensus/accounts?minTotalBalance=999999999999999999999999999'
  'accounts_bad_big_int     /v1/consensus/accounts?minTotalBalance=NA'
  'accounts_extraneous_key  /v1/consensus/accounts?foo=bar'
  'blocks                   /v1/consensus/blocks'
  'block                    /v1/consensus/blocks/8050000'
  'bad_account              /v1/consensus/accounts/oasis1aaaaaaa'
  'account                  /v1/consensus/accounts/oasis1qp0302fv0gz858azasg663ax2epakk5fcssgza7j'
  'runtime-only_account     /v1/consensus/accounts/oasis1qphyxz5csvprhnn09r49nuyzl0jdw0wsj5xpvsg2'
  'delegations              /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/delegations'
  'delegations_to           /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/delegations_to'
  'debonding_delegations    /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/debonding_delegations'
  'debonding_delegations_to /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/debonding_delegations_to'
  # NOTE: entity-related tests are not stable long-term because their output is a combination of
  # the blockchain at a given height (which is stable) and the _current_ metadata_registry state.
  'entities                 /v1/consensus/entities'
  'entity                   /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As='
  'entity_nodes             /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes'
  'node                     /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes/LuIdtuiEPLBJefXVieVruy4kf04jjp5CBJFWVes0ZuE='
  'bad_node                 /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes/NOTANODE'
  'epochs                   /v1/consensus/epochs'
  'epoch                    /v1/consensus/epochs/13403'
  'events                   /v1/consensus/events'
  'proposals                /v1/consensus/proposals'
  'proposal                 /v1/consensus/proposals/2'
  'votes                    /v1/consensus/proposals/2/votes'
  'tx_volume                /v1/consensus/stats/tx_volume'
  'bucket_size              /v1/consensus/stats/tx_volume?bucket_size_seconds=300'
  'nonstandard_bucket_size  /v1/consensus/stats/tx_volume?bucket_size_seconds=301'
  'active_accounts          /v1/consensus/stats/active_accounts'
  'active_accounts_window   /v1/consensus/stats/active_accounts?window_step_seconds=300'
  'active_accounts_emerald  /v1/emerald/stats/active_accounts'
  'txs                      /v1/consensus/transactions'
  'tx                       /v1/consensus/transactions/4c0d40a5db7677667063dfc76d206d0b534dafdf5e072718732ceaf840754ea8'
  'validators               /v1/consensus/validators'
  'validator                /v1/consensus/validators/HPeLbzc88IoYEP0TC4nqSxfxdPCPjduLeJqFvmxFye8='
  'emerald_blocks           /v1/emerald/blocks'
  'emerald_tokens           /v1/emerald/tokens'
  'emerald_txs              /v1/emerald/transactions'
  'emerald_events           /v1/emerald/events'
  'emerald_events_by_type   /v1/emerald/events?type=sdf'
  'emerald_status           /v1/emerald/status'
)
nCases=${#testCases[@]}

# Start the API server.
# Set the timezone (TZ=UTC) to have more consistent outputs across different
# systems, even when not running inside docker.
make oasis-indexer
TZ=UTC ./oasis-indexer --config="${SCRIPT_DIR}/e2e_config.yml" serve &
apiServerPid=$!

# Kill the API server on exit.
trap "kill $apiServerPid" SIGINT SIGTERM EXIT

while ! curl --silent localhost:8008/v1/ >/dev/null; do
  echo "Waiting for API server to start..."
  sleep 1
done

# Run the test cases.
seen=("placeholder") # avoids 'seen[*]: unbound variable' error on zsh
for ((i = 0; i < nCases; i++)); do
  name="$(echo "${testCases[$i]}" | awk '{print $1}')"
  url="$(echo "${testCases[$i]}" | awk '{print $2}')"
  url="$hostname$url"

  # Sanity check: testcase name should be unique
  if [[ " ${seen[*]} " =~ " ${name} " ]]; then
    echo "ERROR: test case $name is not unique"
    exit 1
  fi
  seen+=("$name")

  echo "Running test case: $name"
  # Fetch the server response for $url
  curl --silent --show-error --dump-header "$outDir/$name.headers" "$url" >"$outDir/$name.body"
  # Try to pretty-print and normalize (for stable diffs) the output.
  # If `jq` fails, output was probably not JSON; leave it as-is.
  jq 'if .latest_update? then .latest_update="UNINTERESTING" else . end' \
    <"$outDir/$name.body" \
    >/tmp/pretty 2>/dev/null &&
    cp /tmp/pretty "$outDir/$name.body" || true
  # Sanitize the current timestamp out of the response header so that diffs are stable
  sed -i -E 's/^(Date|Content-Length|Last-Modified): .*/\1: UNINTERESTING/g' "$outDir/$name.headers"
done

diff --recursive "$SCRIPT_DIR/expected" "$outDir" >/dev/null || {
  echo
  echo "NOTE: $SCRIPT_DIR/expected and $outDir differ."
  if [[ $- == *i* ]]; then
    echo "Press enter see the diff, or Ctrl-C to abort."
    read -r
    git diff --no-index "$SCRIPT_DIR"/{expected,actual} || true
    echo
    echo "To re-view the diff, run:"
    echo "  git diff --no-index $SCRIPT_DIR/{expected,actual}"
    echo
    echo "If the new results are expected, re-run this script after copying the new results into .../expected:"
    echo "  make accept-e2e-regression"
  else
    # Running in script mode (likely in CI)
    echo "CI diff:"
    git diff --no-index "$SCRIPT_DIR"/{expected,actual} || true
  fi
  exit 1
}

echo
echo "E2E regression tests passed!"
