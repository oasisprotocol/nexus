#!/bin/bash

# This script is a simple e2e regression test for Nexus.
#
# It takes the name of a test suite, and
#  - builds nexus (if -b is given) and indexes (if -a is given) the block
#    range defined by the suite
#  - runs a fixed set of URLs against the HTTP API
#  - runs a fixed set of SQL queries against the DB
# and saves the responses to files, then check that the responses match
# the expected outputs (from a previous run).
#
# If the differences are expected, simply check the new responses into git.

set -euo pipefail

E2E_REGRESSION_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

build=
analyze=
while getopts ba opt; do
  case $opt in
    b) build=1;;
    a) analyze=1;;
  esac
done
shift "$((OPTIND - 1))"

# Read arg
suite="${1:-}"
TEST_DIR="$E2E_REGRESSION_DIR/$suite"
if [[ -z "$suite" || ! -e "$TEST_DIR/e2e_config_1.yml" ]]; then
  cat >&2 <<EOF
Usage: $0 [-ba] <suite>

  -b  Build nexus
  -a  Run analysis steps
EOF
  exit 1
fi

# Build
if [[ -n "$build" ]]; then
  make nexus
fi

# Analyze
if [[ -n "$analyze" ]]; then
  ./nexus --config "$TEST_DIR/e2e_config_1.yml" analyze
  ./nexus --config "$TEST_DIR/e2e_config_2.yml" analyze
  echo "*** Analyzers finished; starting api tests..."
fi

# Load test cases
source "$TEST_DIR/test_cases.sh"

# The hostname of the API server to test
hostname="http://localhost:8008"

# The command to invoke psql (complete with connection params)
# HACK: Assuming `make` returns a docker command, sed removes -i and -t flags because we'll be running without a TTY.
psql="$(make --dry-run --no-print-directory psql | sed -E 's/ -(it|ti|i|t) / /g')"

# The directory to store the actual responses in
outDir="$TEST_DIR/actual"
mkdir -p "$outDir"
rm "$outDir"/* || true

nCases=${#testCases[@]}

# Start the API server.
# Set the timezone (TZ=UTC) to have more consistent outputs across different
# systems, even when not running inside docker.
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
    #
    # .first_activity is ignored, because its not deterministic between fast-sync and slow-sync.
    #  - this is due the fact that we start tests at the middle of the chain and the first activity
    #    for accounts depends on when the genesis is processed. Fast-sync only fetches the genesis
    #    at the height when slow-sync starts, so the first_activity for accounts in genesis is different.
    if [[ "$param" = *_raw ]]; then
      # _raw endpoints return non-JSON responses, we don't sanitize those.
      echo "Skipping non JSON file: $outDir/$name.body"
    else
      jq \
        ' (if .latest_update_age_ms? then .latest_update_age_ms="UNINTERESTING" else . end) |
          ((.evm_nfts?[]? | select(.metadata_accessed) | .metadata_accessed) = "UNINTERESTING") |
          (if .metadata_accessed? then .metadata_accessed = "UNINTERESTING" else . end) |
          (if .first_activity? then .first_activity = "NONDETERMINISTIC" else . end) |
          ((.accounts?[]? | select(.first_activity) | .first_activity) = "NONDETERMINISTIC")' \
        <"$outDir/$name.body" \
        >/tmp/pretty 2>/dev/null &&
        cp /tmp/pretty "$outDir/$name.body"
    fi

    # Sanitize the current timestamp out of the response header so that diffs are stable
    # Use a temp file for compatibility with both macOS and Linux sed
    sed -E 's/^(Date|Content-Length|Last-Modified): .*/\1: UNINTERESTING/g' "$outDir/$name.headers" > "$outDir/$name.headers.tmp" && mv "$outDir/$name.headers.tmp" "$outDir/$name.headers"
  else
    # $param is a SQL query; fetch the DB response
    $psql -A -o /dev/stdout -c "COPY ($param) TO STDOUT CSV HEADER" | sed $'s/\r$//' >"$outDir/$name.csv" \
      || { cat "$outDir/$name.csv"; exit 1; }  # psql prints the error message on stdout
  fi
done

diff --recursive "$TEST_DIR/expected" "$outDir" >/dev/null || {
  echo
  echo "NOTE: $TEST_DIR/expected and $outDir differ."
  # The expected files contain a symlink, which 'git diff' cannot follow (but regular 'diff' can).
  # Create a copy of the `expected` dir with the symlink contents materialized; we'll diff against that.
  expectedDerefDir="/tmp/nexus-e2e-expected-$suite"
  rm -rf "$expectedDerefDir"
  # Use -L for macOS compatibility (equivalent to --dereference on Linux)
  cp -rL "$TEST_DIR/expected" "$expectedDerefDir"
  if [[ -t 1 ]]; then # Running in a terminal
    echo "Press enter see the diff, or Ctrl-C to abort."
    read -r
  else
    # Running outside a terminal (likely in CI)
    echo "CI diff:"
  fi
  git diff --no-index "$expectedDerefDir" "$outDir" || true
  if [[ -t 1 ]]; then # Running in a terminal
    echo
    echo "To re-view the diff, run:"
    echo "  git diff --no-index $expectedDerefDir $outDir"
    echo
    echo "If the new results are expected, copy the new results into .../expected:"
    echo "  $E2E_REGRESSION_DIR/accept.sh $suite"
  fi
  exit 1
}

echo
echo "E2E regression test suite \"$suite\" passed!"
