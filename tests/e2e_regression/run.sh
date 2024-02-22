#!/bin/bash

# This script is a simple e2e regression test for Nexus.
#
# It runs
#  - a fixed set of URLs against the HTTP API
#  - a fixed set of SQL queries against the DB
# and saves the responses to files, then check that the responses match
# the expected outputs (from a previous run).
#
# If the differences are expected, simply check the new responses into git.
#
# NOTE: It is the responsibility of the caller to invoke this script against a
# DB that is SYNCED TO THE SAME HEIGHT as in the run that produced the
# expected outputs.

set -euo pipefail

# Read arg
suite="${1:-}"
if [[ "$suite" != "eden" &&  "$suite" != "damask"  ]]; then
  echo "Usage: $0 [eden|damask]"
  exit 1
fi

# Load test cases
source tests/e2e_regression/test_cases.sh
testCasesName="${suite}TestCases[@]"
testCases=( "${!testCasesName}" )

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)/$suite"

# The hostname of the API server to test
hostname="http://localhost:8008"

# The command to invoke psql (complete with connection params)
# HACK: Assuming `make` returns a docker command, sed removes -i and -t flags because we'll be running without a TTY.
psql="$(make --dry-run --no-print-directory psql | sed -E 's/\s-i?ti?\s/ /')"

# The directory to store the actual responses in
outDir="$TEST_DIR/actual"
mkdir -p "$outDir"
rm "$outDir"/* || true

nCases=${#testCases[@]}

# Start the API server.
# Set the timezone (TZ=UTC) to have more consistent outputs across different
# systems, even when not running inside docker.
make nexus
TZ=UTC ./nexus --config="${TEST_DIR}/e2e_config_1.yml" serve &
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
        ((.evm_nfts?[]? | select(.metadata_accessed) | .metadata_accessed) = "UNINTERESTING") |
        (if .metadata_accessed? then .metadata_accessed = "UNINTERESTING" else . end)
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

diff --recursive "$TEST_DIR/expected" "$outDir" >/dev/null || {
  echo
  echo "NOTE: $TEST_DIR/expected and $outDir differ."
  {
    # The expected files contain a symlink, which 'git diff' cannot follow (but regular 'diff' can).
    # Create a copy of the `expected` dir with the symlink contents materialized; we'll diff against that.
    rm -rf /tmp/nexus-e2e-expected; cp -r --dereference "$TEST_DIR/expected" /tmp/nexus-e2e-expected;
  }
  if [[ -t 1 ]]; then # Running in a terminal
    echo "Press enter see the diff, or Ctrl-C to abort."
    read -r
    git diff --no-index /tmp/nexus-e2e-expected "$TEST_DIR/actual" || true
    echo
    echo "To re-view the diff, run:"
    echo "  git diff --no-index /tmp/nexus-e2e-expected $TEST_DIR/actual"
  else
    # Running outside a terminal (likely in CI)
    echo "CI diff:"
    git diff --no-index "$TEST_DIR"/{expected,actual} || true
  fi
  echo
  echo "If the new results are expected, copy the new results into .../expected:"
  echo "  make accept-e2e-regression"
  exit 1
}

echo
echo "E2E regression tests passed!"
