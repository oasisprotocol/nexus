#!/bin/bash

# For each of the chains, fetch all Blockscout-verified contracts' addresses and
# then attempt to import them into Sourcify one by one.

chains=(23294 23295 42262 42261)
for chainId in "${chains[@]}"; do
  addrsFile="${chainId}.addrs"
  case $chainId in
    42262) blockscoutApi=https://old-explorer.emerald.oasis.io/api ;;
    42261) blockscoutApi=https://testnet.old-explorer.emerald.oasis.io/api ;;
    23294) blockscoutApi=https://old-explorer.sapphire.oasis.io/api ;;
    23295) blockscoutApi=https://testnet.old-explorer.sapphire.oasis.io/api ;;
  esac

  if ! [[ -f $addrsFile ]]; then
    # Fetch all verified contracts' addresses
    page=0
    while true; do
      (( page++ ))
      echo "Fetching page $page"
      addrs=$(curl -sS "${blockscoutApi}?module=contract&action=listcontracts&filter=verified&page=${page}" | jq -r '.result[] | .Address')
      if [[ -z "$addrs" ]]; then break; fi
      echo "$addrs" >>$addrsFile
    done
  fi

  for addr in $(cat $addrsFile); do
    if grep -q $addr "${chainId}.succeeded" 2>/dev/null; then continue; fi
    echo "Processing $addr for chain $chainId"
    chainId=$chainId blockscoutApi=$blockscoutApi ./split-sol.sh $addr \
      && { echo $addr >>"${chainId}.succeeded"; } \
      || { echo $addr >>"${chainId}.failed"; cat "${chainId}.failed" | sort | uniq >/tmp/failsort; mv /tmp/failsort "${chainId}.failed"; }
  done
done
