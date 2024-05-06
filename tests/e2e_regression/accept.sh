#!/bin/bash

set -euo pipefail

E2E_REGRESSION_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# Read arg
suite="${1:-}"
TEST_DIR="$E2E_REGRESSION_DIR/$suite"
if [[ -z "$suite" || ! -e "$TEST_DIR/e2e_config_1.yml" ]]; then
  echo >&2 "Usage: $0 <suite>"
  exit 1
fi

[[ -L "$TEST_DIR/expected" ]] && {
  echo >&2 "$suite/expected is a symbolic link to $(readlink "$TEST_DIR/expected")."
  echo >&2 "Use $0 on that suite instead."
  exit 1
}

[[ -d "$TEST_DIR/actual" ]] || {
  echo "Note: No actual outputs found for suite $suite. Nothing to accept."
  exit 0
}
# Delete all old expected files first, in case any test cases were renamed or removed.
rm -rf "$TEST_DIR/expected"
# Copy the actual outputs to the expected outputs.
cp -r  "$TEST_DIR/"{actual,expected}
# The result of the "spec" test is a special case. It should always match the
# current openAPI spec file, so we symlink it to avoid having to update the expected
# output every time the spec changes.
rm -f "$TEST_DIR/expected/spec.body"
ln -s  ../../../../api/spec/v1.yaml "$TEST_DIR/expected/spec.body"
