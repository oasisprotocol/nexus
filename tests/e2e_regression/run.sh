#!/bin/bash

# This script is a simple e2e regression test for Nexus.
#
# It runs
#  - a fixed set of URLs against the HTTP API
#  - a fixed set of SQL queries against the DB
# and saves the responses to files, then check that the responses match
# the expected outputs (from a previous run).
#
# If the differences are expected, smiply check the new responses into git.
#
# NOTE: It is the responsibility of the caller to invoke this script against a
# DB that is SYNCED TO THE SAME HEIGHT as in the run that produced the
# expected outputs.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# The hostname of the API server to test
hostname="http://localhost:8008"

# The command to invoke psql (complete with connection params)
# HACK: Assuming `make` returns a docker command, sed removes -i and -t flags because we'll be running without a TTY.
psql="$(make --dry-run --no-print-directory psql | sed -E 's/\s-i?ti?\s/ /')"

# The directory to store the actual responses in
outDir="$SCRIPT_DIR/actual"
mkdir -p "$outDir"
rm "$outDir"/* || true

# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.
testCases=(
  ## Consensus.
  'db__allowances               select * from chain.allowances order by beneficiary, owner'
  'db__debonding_delegations    select debond_end, delegator, delegatee, shares from chain.debonding_delegations order by debond_end, delegator, delegatee, shares' # column `id` is internal use only, and not stable
  'db__delegations              select * from chain.delegations where shares != 0 order by delegatee, delegator'
  'db__epochs                   select * from chain.epochs order by id'
  'db__entities                 select * from chain.entities order by id'
  'db__events                   select tx_block, tx_index, tx_hash, type, ARRAY(SELECT unnest(related_accounts) ORDER BY 1) AS sorted_related_accounts, body::text from chain.events order by tx_block, tx_index, type, body::text'
  'db__nodes                    select id, entity_id, roles, expiration, voting_power from chain.nodes order by id'
  'db__runtime_nodes            select rn.*, n.roles FROM chain.runtime_nodes rn LEFT JOIN chain.nodes n ON (rn.node_id = n.id) ORDER BY runtime_id, node_id'
  ## Runtimes.
  'db__account_related_txs      select * from chain.runtime_related_transactions order by runtime, tx_round, tx_index, account_address'
  'db__runtime_accounts         select * from chain.runtime_accounts order by runtime, address'
  'db__runtime_transfers        select * from chain.runtime_transfers order by runtime, round, sender, receiver'
  'db__contract_gas_use         select c.runtime, contract_address, (SELECT gas_for_calling FROM chain.runtime_accounts ra WHERE (ra.runtime = c.runtime) AND (ra.address = c.contract_address)) AS gas_used, timestamp as created_at from chain.evm_contracts c left join chain.runtime_transactions rt on (c.creation_tx = rt.tx_hash) order by runtime, contract_address'
  # sdk_balances, evm_balances: Do not query zero balances; whether they are stored depends on indexing order and fast-sync.
  'db__sdk_balances             select * from chain.runtime_sdk_balances where balance != 0 order by runtime, account_address'
  'db__evm_balances             select * from chain.evm_token_balances where balance != 0 order by runtime, token_address, account_address'
  'db__evm_tokens               select runtime, token_address, token_type, token_name, symbol, decimals, total_supply, num_transfers from chain.evm_tokens order by token_address'
  'db__evm_contracts            select runtime, contract_address, creation_tx, md5(abi::text) as abi_md5 from chain.evm_contracts order by runtime, contract_address'

  'status                         /v1/'
  'spec                           /v1/spec/v1.yaml'
  'trailing_slash                 /v1/consensus/accounts/?limit=1'
  'accounts                       /v1/consensus/accounts'
  'min_balance                    /v1/consensus/accounts?minTotalBalance=1000000'
  'big_int_balance                /v1/consensus/accounts?minTotalBalance=999999999999999999999999999'
  'accounts_bad_big_int           /v1/consensus/accounts?minTotalBalance=NA'
  'accounts_extraneous_key        /v1/consensus/accounts?foo=bar'
  'blocks                         /v1/consensus/blocks'
  'block                          /v1/consensus/blocks/8050000'
  'bad_account                    /v1/consensus/accounts/oasis1aaaaaaa'
  'account                        /v1/consensus/accounts/oasis1qp0302fv0gz858azasg663ax2epakk5fcssgza7j'
  'runtime-only_account           /v1/consensus/accounts/oasis1qphyxz5csvprhnn09r49nuyzl0jdw0wsj5xpvsg2'
  'delegations                    /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/delegations'
  'delegations_to                 /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/delegations_to'
  'debonding_delegations          /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/debonding_delegations'
  'debonding_delegations_to       /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/debonding_delegations_to'
  # NOTE: entity-related tests are not stable long-term because their output is a combination of
  #       the blockchain at a given height (which is stable) and the _current_ metadata_registry state.
  #       We circumvent this by not fetching from metadata_registry at all, so the same metadata (= none) is always present for the test.
  'entities                       /v1/consensus/entities'
  'entity                         /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As='
  'entity_nodes                   /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes'
  'node                           /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes/LuIdtuiEPLBJefXVieVruy4kf04jjp5CBJFWVes0ZuE='
  'bad_node                       /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes/NOTANODE'
  'epochs                         /v1/consensus/epochs'
  'epoch                          /v1/consensus/epochs/13403'
  'events                         /v1/consensus/events'
  'proposals                      /v1/consensus/proposals'
  'proposal                       /v1/consensus/proposals/2'
  'votes                          /v1/consensus/proposals/2/votes'
  'tx_volume                      /v1/consensus/stats/tx_volume'
  'window_size                    /v1/consensus/stats/tx_volume?window_size_seconds=300&window_step_seconds=300'
  'nonstandard_window_size        /v1/consensus/stats/tx_volume?window_size_seconds=301&window_step_seconds=300'
  'active_accounts                /v1/consensus/stats/active_accounts'
  'active_accounts_window         /v1/consensus/stats/active_accounts?window_step_seconds=300'
  'active_accounts_emerald        /v1/emerald/stats/active_accounts'
  'txs                            /v1/consensus/transactions'
  'tx                             /v1/consensus/transactions/4c0d40a5db7677667063dfc76d206d0b534dafdf5e072718732ceaf840754ea8'
  'validators                     /v1/consensus/validators'
  'validator                      /v1/consensus/validators/HPeLbzc88IoYEP0TC4nqSxfxdPCPjduLeJqFvmxFye8='
  'emerald_blocks                 /v1/emerald/blocks'
  'emerald_txs                    /v1/emerald/transactions'
  'emerald_tx                     /v1/emerald/transactions/a6471a9c6f3307087586da9156f3c9876fbbaf4b23910cd9a2ac524a54d0aefe'
  'emerald_failed_tx              /v1/emerald/transactions/a7e76442c52a3cb81f719bde26c9a6179bd3415f96740d91a93ee8f205b45150'
  'emerald_events                 /v1/emerald/events'
  'emerald_events_by_type         /v1/emerald/events?type=sdf'
  'emerald_tokens                 /v1/emerald/evm_tokens'
  'emerald_token                  /v1/emerald/evm_tokens/oasis1qpgcp5jzlgk4hcenaj2x82rqk8rrve2keyuc8aaf'
  'emerald_token_holders          /v1/emerald/evm_tokens/oasis1qpgcp5jzlgk4hcenaj2x82rqk8rrve2keyuc8aaf/holders'
  'emerald_account_with_rose      /v1/emerald/accounts/oasis1qrt0sv2s2x2lkt9e7kmr2mzxgme8m0pzauwztprl'
  'emerald_account_with_evm_token /v1/emerald/accounts/oasis1qpwx3ptmvcceqkd4syjmqf9jmdlf90xmuuy0f6y9'
  'emerald_contract_account       /v1/emerald/accounts/oasis1qrrmuaed6numjju8gajzn68tn2edlvycjc50nfva'
  'emerald_status                 /v1/emerald/status'
  'emerald_tx_volume              /v1/emerald/stats/tx_volume'
)
nCases=${#testCases[@]}

# Start the API server.
# Set the timezone (TZ=UTC) to have more consistent outputs across different
# systems, even when not running inside docker.
make nexus
TZ=UTC ./nexus --config="${SCRIPT_DIR}/e2e_config_1.yml" serve &
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
  name="$(echo "${testCases[$i]}" | cut -d' ' -f1)"
  param="$(echo "${testCases[$i]}" | cut -d' ' -f2- | sed 's/^ *//')" # URL or SQL query

  # Sanity check: testcase name should be unique
  if [[ " ${seen[*]} " =~ " ${name} " ]]; then
    echo "ERROR: test case $name is not unique"
    exit 1
  fi
  seen+=("$name")

  echo "Running test case: $name"
  if [[ "$param" = /v1* ]]; then
    # $param is an URL; fetch the server response
    url="$hostname$param"
    curl --silent --show-error --dump-header "$outDir/$name.headers" "$url" >"$outDir/$name.body"
    # Try to pretty-print and normalize (for stable diffs) the output.
    # If `jq` fails, output was probably not JSON; leave it as-is.
    jq \
      '
        (if .latest_update_age_ms? then .latest_update_age_ms="UNINTERESTING" else . end) |
        ((.evm_nfts?[]? | select(.metadata_accessed) | .metadata_accessed) = "UNINTERESTING")
      ' \
      <"$outDir/$name.body" \
      >/tmp/pretty 2>/dev/null &&
      cp /tmp/pretty "$outDir/$name.body" || true
    # Sanitize the current timestamp out of the response header so that diffs are stable
    sed -i -E 's/^(Date|Content-Length|Last-Modified): .*/\1: UNINTERESTING/g' "$outDir/$name.headers"
  else
    # $param is a SQL query; fetch the DB response
    $psql -A -o /dev/stdout -c "COPY ($param) TO STDOUT CSV HEADER" | sed $'s/\r$//' >"$outDir/$name.csv" \
      || { cat "$outDir/$name.csv"; exit 1; }  # psql prints the error message on stdout
  fi
done

diff --recursive "$SCRIPT_DIR/expected" "$outDir" >/dev/null || {
  echo
  echo "NOTE: $SCRIPT_DIR/expected and $outDir differ."
  {
    # The expected files contain a symlink, which 'git diff' cannot follow (but regular 'diff' can).
    # Create a copy of the `expected` dir with the symlink contents materialized; we'll diff against that.
    rm -rf /tmp/nexus-e2e-expected; cp -r --dereference "$SCRIPT_DIR/expected" /tmp/nexus-e2e-expected;
  }
  if [[ -t 1 ]]; then # Running in a terminal
    echo "Press enter see the diff, or Ctrl-C to abort."
    read -r
    git diff --no-index /tmp/nexus-e2e-expected "$SCRIPT_DIR/actual" || true
    echo
    echo "To re-view the diff, run:"
    echo "  git diff --no-index /tmp/nexus-e2e-expected $SCRIPT_DIR/actual"
  else
    # Running outside a terminal (likely in CI)
    echo "CI diff:"
    git diff --no-index "$SCRIPT_DIR"/{expected,actual} || true
  fi
  echo
  echo "If the new results are expected, copy the new results into .../expected:"
  echo "  make accept-e2e-regression"
  exit 1
}

echo
echo "E2E regression tests passed!"
