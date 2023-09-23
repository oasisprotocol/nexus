#!/bin/bash

set -euo pipefail

# Our e2e regression test runs the analyzer twice in close succession. This is a slightly hacky but
# simple way to ensure block analyzers run first, and non-block analyzers perform EVM queries always
# at the same height, thereby hitting the offline response cache.
#
# This script compares the key parameters of the two config files used in the two runs. If any of
# those parameters differ, it shows the diff and exits with an error.

# Element of the config files that we'll compare
important_attrs='{"cache": .analysis.source.cache.cache_dir, "chain_name": .analysis.source.chain_name, "db": .analysis.storage.endpoint}'

# Enables aliases to work in non-interactive shells.
shopt -s expand_aliases

# A converter whose only dependency is python3, which is likely preinstalled
alias yaml2json="python3 -c 'import sys,yaml,json; print(json.dumps(yaml.safe_load(str(sys.stdin.read()))))'" 

# Compare
cat tests/e2e_regression/e2e_config_1.yml | yaml2json | jq "$important_attrs" > /tmp/e2e_config_1.summary
cat tests/e2e_regression/e2e_config_2.yml | yaml2json | jq "$important_attrs" > /tmp/e2e_config_2.summary
diff /tmp/e2e_config_1.summary /tmp/e2e_config_2.summary || { echo "The two config files for e2e tests differ in key parameters! See diff above."; exit 1; }
