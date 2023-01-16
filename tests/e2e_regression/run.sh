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
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

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
  'bad_big_int              /v1/consensus/accounts?minTotalBalance=NA'
  'extraneous_key           /v1/consensus/accounts?foo=bar'
  'blocks                   /v1/consensus/blocks'
  'block                    /v1/consensus/blocks/8050000'
  'bad_account              /v1/consensus/accounts/oasis1aaaaaaa'
  'account                  /v1/consensus/accounts/oasis1qp0302fv0gz858azasg663ax2epakk5fcssgza7j'
  'runtime-only_account     /v1/consensus/accounts/oasis1qphyxz5csvprhnn09r49nuyzl0jdw0wsj5xpvsg2'
  'delegations              /v1/consensus/accounts/oasis1qp0302fv0gz858azasg663ax2epakk5fcssgza7j/delegations'
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
  'txs                      /v1/consensus/transactions'
  'tx                       /v1/consensus/transactions/4c0d40a5db7677667063dfc76d206d0b534dafdf5e072718732ceaf840754ea8'
  'validators               /v1/consensus/validators'
  'validator                /v1/consensus/validators/HPeLbzc88IoYEP0TC4nqSxfxdPCPjduLeJqFvmxFye8='
  'emerald_blocks           /v1/emerald/blocks'
  'emerald_tokens           /v1/emerald/tokens'
  'emerald_txs              /v1/emerald/transactions'
  'emerald_events           /v1/consensus/events'
  'emerald_events_by_type   /v1/consensus/events?type=sdf'
)
nCases=${#testCases[@]}

seen=()

for (( i=0; i<nCases; i++ )); do
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
  curl --silent --show-error --dump-header "$outDir/$name.headers" "$url" > "$outDir/$name.body"
  # Try to pretty-print the output. If `jq` fails, output was probably not JSON; leave it as-is.
  <"$outDir/$name.body" jq >/tmp/pretty 2>/dev/null && cp /tmp/pretty "$outDir/$name.body" || true
  # Sanitize the current timestamp out of the response header so that diffs are stable
  sed -i 's/^Date: .*/Date: UNINTERESTING/g' "$outDir/$name.headers"
  sed -i 's/^Content-Length: .*/Content-Length: UNINTERESTING/g' "$outDir/$name.headers"
done

diff --recursive "$SCRIPT_DIR/expected" "$outDir" || {
  echo
  echo "ERROR: $SCRIPT_DIR/expected and $outDir differ (see above)."
  echo "If the new reults are expected, copy them into .../expected and re-run this script."
  exit 1
}

echo
echo "E2E regression tests passed!"
