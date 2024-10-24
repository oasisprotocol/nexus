#!/bin/bash
# shellcheck disable=SC2046,SC2044  # quick-n-dirty, for using unquoted `find` output as part of commands

# This script vendors (=clones) type definitions from a given version of
# oasis-core into a local directory, coreapi/$VERSION.
#
# The intention is to use them to communicate with archive nodes that use
# old oasis-core. (Nexus needs to be able to talk to archive nodes
# and the current node; ie cannot directly include multiple versions of
# oasis-core as a dependency; Go does not support that.)
#
# WHEN TO USE:
# We expect to need it very rarely; only when the gRPC API of the node
# (which nexus uses to communicate with the node) changes.
# The gRPC protocol is NOT VERSIONED (!), so technically we'd need to
# deep-read the oasis-core release notes for every release to see if
# the gRPC API changed. In practice, it's strongly correlated with
# the consensus version (listed on top of release notes). Also in practice,
# we needed to vendor types exactly once for each named release
# (Beta, Cobalt, Damask, etc).
#
# HOW TO USE:
# 1) Set an appropriate VERSION below. Run the script.
# 2) Import the new types into the nexus codebase. Compile.
# 3) Manually fix any issues that arise. THIS SCRIPT IS FAR FROM FOOLPROOF;
#    it is a starting point for vendoring a reasonable subset of oasis-core.
#    Expand the "manual patches" section below; or don't, and just commit
#    whatever manual fixes. You only need to vendor once. Patches are nice
#    because they document the manual fixes/hacks/differences from oasis-core.

set -euo pipefail

VERSION="${1:-v22.2.11}" # Damask
MODULES=(beacon consensus genesis governance keymanager registry roothash scheduler staking)
if [[ $VERSION == v22.* ]]; then MODULES+=(runtime/client); fi
if [[ $VERSION == v22.* ]] || [[ $VERSION == v23.* ]] || [[ $VERSION == v24.* ]]; then MODULES+=(upgrade); fi
if [[ $VERSION == v24.* ]]; then MODULES+=(vault); fi

OUTDIR="coreapi/$VERSION"

echo "Vendoring oasis-core $VERSION into $OUTDIR"

# Copy oasis-core
(
  cd ../oasis-core
  output=$(git status --untracked-files=no --porcelain)
  if [[ "$output" != "" ]]; then
    echo "WARNING: oasis-core is not clean, will not continue:"
    echo "$output"
    exit 1
  fi
  git checkout "$VERSION" # "850373a2d" # master as of 2023-10-03
)
rm -rf "$OUTDIR"
for m in "${MODULES[@]}"; do
  mkdir -p "$OUTDIR/$m"
  cp -r "../oasis-core/go/$m/api" "$OUTDIR/$m"
done
cp -r ../oasis-core/go/consensus/genesis "$OUTDIR/consensus"
mkdir -p "$OUTDIR/common/node"
cp ../oasis-core/go/common/node/*.go "$OUTDIR/common/node"

# Copy keymanager/secrets package for v24, and fix imports.
if [[ $VERSION == v24.* ]]; then
  cp -r ../oasis-core/go/keymanager/secrets "$OUTDIR/keymanager/secrets"
  cp -r ../oasis-core/go/keymanager/churp "$OUTDIR/keymanager/churp"
fi

rm $(find "$OUTDIR/" -name '*_test.go')

# Fix imports: References to the "real" oasis-core must now point to the vendored coutnerpart.
modules_or=$(IFS="|"; echo "${MODULES[*]}")
sed -E -i "s#github.com/oasisprotocol/oasis-core/go/($modules_or)/api(/[^\"]*)?#github.com/oasisprotocol/nexus/$OUTDIR/\\1/api\\2#" $(find "$OUTDIR/" -type f)
sed -E -i "s#github.com/oasisprotocol/oasis-core/go/common/node#github.com/oasisprotocol/nexus/$OUTDIR/common/node#" $(find "$OUTDIR/" -type f)
sed -E -i "s#github.com/oasisprotocol/oasis-core/go/consensus/genesis#github.com/oasisprotocol/nexus/$OUTDIR/consensus/genesis#" $(find "$OUTDIR/" -type f)
sed -E -i "s#github.com/oasisprotocol/oasis-core/go/keymanager/secrets#github.com/oasisprotocol/nexus/$OUTDIR/keymanager/secrets#" $(find "$OUTDIR/" -type f)
sed -E -i "s#github.com/oasisprotocol/oasis-core/go/keymanager/churp#github.com/oasisprotocol/nexus/$OUTDIR/keymanager/churp#" $(find "$OUTDIR/" -type f)

# Remove functions and interfaces. We only need the types.
for f in $(find "$OUTDIR/" -name "*.go" -type f | sort); do
  scripts/vendor-oasis-core/remove_func.py <"$f" >/tmp/nofunc || { echo "Failed to remove functions from $f"; exit 1; }
  mv /tmp/nofunc "$f"
done

# Clean up
gofumpt -w "$OUTDIR/"
goimports -w "$OUTDIR/"

# Apply manual patches
# None

# 2) Reuse the Address struct from oasis-core.
>"$OUTDIR/staking/api/address.go" cat <<EOF
package api

import (
        original "github.com/oasisprotocol/oasis-core/go/staking/api"
)

type Address = original.Address
EOF
# 3) Reuse EpochTime from oasis-core, and some other minor fixes.
for p in scripts/vendor-oasis-core/patches/"$VERSION"/*.patch; do
  echo "Applying patch $p"
  git apply "$p"
done

# Check that no unexpected direct oasis-core imports are left,
# now that we've removed non-API code and minimized imports.
whitelisted_imports=(
  github.com/oasisprotocol/oasis-core/go/common
  github.com/oasisprotocol/oasis-core/go/storage
  github.com/oasisprotocol/oasis-core/go/upgrade
)
surprising_core_imports="$(
  grep --no-filename github.com/oasisprotocol/oasis-core $(find "$OUTDIR/" -type f) \
  | grep -v 'original' `# we introduced this dependency intentionally; see above` \
  | grep -oE '"github.com/oasisprotocol/oasis-core/[^"]*"' \
  | sort \
  | uniq \
  | grep -vE "$(IFS="|"; echo "${whitelisted_imports[*]}")" \
  || true
)"
if [[ "$surprising_core_imports" != "" ]]; then
  echo "WARNING: Unexpected direct oasis-core mentions remain in the code:"
  echo "$surprising_core_imports"
  exit 1
else
  echo "No unexpected oasis-core imports remain in the code."
fi
