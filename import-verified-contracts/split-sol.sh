#!/bin/bash

set -euo pipefail

# Converts the info of a verified-contract from blockscout dump to something
# that is (maybe) importable to sourcify, and attempts to auto-import into sourcify.
#
# USAGE: <this>.sh <contract_address>
#
# OUTPUT/EFFECTS:
#  - Just diagnostics on stdout
#  - Creates various files in /tmp/<addr> that are used for the import to Sourcify
#
# USEFUL BLOCKSCOUT LINKS:
# To obtain the dump from blockscout:
#   curl 'https://old-explorer.emerald.oasis.io/api?module=contract&action=getsourcecode&address=0xb36afae1ab1fb4e659633f962ad94eeabe819e54'
# To list all verified contracts:
#   https://old-explorer.emerald.oasis.io/api?module=contract&action=listcontracts&filter=verified   (&page=2, &page=3 etc)
# Blockscout API reference:
#   https://docs.blockscout.com/for-users/api/rpc-endpoints/contract

# Contract to verify
addr="$1"
[[ "$addr" =~ ^0x[0-9a-fA-F]{40}$ ]] || { echo "Invalid address: $addr, must be '0x' and 40 hex chars'"; exit 1; }

# Let these be provided externally but default to Emerald mainnet.
chainId=${chainId:-42262}
blockscoutApi=${blockscoutApi:-https://old-explorer.emerald.oasis.io/api}

SOURCIFY_SERVER=https://sourcify.dev/server
# SOURCIFY_SERVER=http://localhost:5555  # For debugging with https://docs.sourcify.dev/docs/running-server/
# To run sourcify server locally, use the debug build. Check out the sourcify repo and run:
#    npx lerna run build && docker run -it -p 9229:9229 -p 5555:5555 -p 10000:10000 --volume /home/mitjat/projects/sourcify:/home/app sourcify-server-debug

# Fetch contract info from blockscout
input_file=/tmp/$addr/blockscout.json
if ! [[ -f $input_file ]]; then
  rm -rf /tmp/$addr
  mkdir -p $(dirname $input_file)
  curl -sS "${blockscoutApi}?module=contract&action=getsourcecode&address=$addr" >$input_file
fi 

# Computes keccak256 hash of stdin.
# Requires "npm i -g keccak256"
keccak256() {
  node -e "process.stdin.resume().setEncoding('utf8'); let data=''; process.stdin.on('data', chunk => { data += chunk; }); process.stdin.on('end', () => { console.log(require('keccak256')(data).toString('hex')); });"
}

cd "$(dirname $input_file)"

# Extract concatenated sources
cat $input_file | jq -r '.result[0].SourceCode' | tr -d $'\r' >sources.sol
if ! grep -qE '^// File:' sources.sol; then
  echo '// File: main.sol' >sources.sol
  cat $input_file | jq -r '.result[0].SourceCode' | tr -d $'\r' >>sources.sol
fi


# Split sources into individual files.
# filenames=()
# while IFS= read -r line; do
#     # Check if line starts with "File: "
#     if [[ $line == "// File: "* ]]; then
#         # Extract filename
#         filename=$(echo $line | cut -c10-)
#         filenames=(${filenames[@]} $filename)
#         # Start writing to new file
#         echo "Creating file: $filename"
# 	      mkdir -p "$(dirname $filename)"
#         echo "$line" > "$filename"
#     else if [[ -z "${filename:-}" ]]; then continue; fi
#         # Write contents to current file
#         echo "$line" >> "$filename"
#     fi
# done <sources.sol
filenames=(sources.sol)  # Split files don't work because they lack import directives/pointers to each other. Do not use the individual files, just the concatenated one.

# Prepare data for metadata.json
{
echo "{"
first=1
for filename in ${filenames[@]}; do
  if [[ $first == 0 ]]; then echo ","; else first=0; fi
  echo "\"$filename\": {\"keccak256\": \"0x$(cat $filename | keccak256)\", \"content\": $(cat $filename | jq -Rs)}"
done
echo "}"
} >/tmp/sources.json

mainContract="$(jq -r <$input_file '.result[0].ContractName')"
entryFile=$(grep -lE "^contract $mainContract" "${filenames[@]}") || entryFile="sources.sol"  # entryFile is informative only and not passed to Sourcify; we're submitting a single concatenated source file anyway
echo "Entrypoint for ${addr} determined as: contract \`$mainContract\` in \"$entryFile\"."

compilerVersion="$(jq <$input_file -r '.result[0].CompilerVersion')"
evmVersion=$(jq -r <$input_file '.result[0].EVMVersion')
if [[ "$evmVersion" == "default" ]]; then
	echo "WARNING: EVM version is 'default', guessing the version."
	evmVersion="$(curl -sS https://raw.githubusercontent.com/ethereum/solidity/$(echo $compilerVersion | cut -d+ -f1)/liblangutil/EVMVersion.h | grep 'case Version::' | cut -d'"' -f2 | tail -n1)" \
		|| evmVersion="homestead"	# fallback; for very old versions, the EVMVersion.h file is not available
	echo "  Guessed EVM version: $evmVersion"
fi
optimizerEnabled="$(jq <$input_file -r '.result[0].OptimizationUsed == "true"')"  # string "true" or "false", unquoted
[[ "$optimizerEnabled" == "true" || "$optimizerEnabled" == "false" ]] || { echo "Invalid value for .result[0].OptimizationUsed: $optimizerEnabled"; exit 1; }

# Fill a metadata.json template.
jq ".output.abi = $(jq <$input_file '.result[0].ABI' -r) | .settings.compilationTarget = {\"${entryFile}\": \"${mainContract}\"}" >metadata.json <<EOF
{
	"compiler": {
		"version": "$compilerVersion"
	},
	"language": "Solidity",
	"output": {
		"devdoc": {
			"kind": "dev",
			"methods": {},
			"version": 1
		},
		"userdoc": {
			"kind": "user",
			"methods": {},
			"version": 1
		}
	},
	"settings": {
		"compilationTarget": {
			"contracts/contract.sol": "DTEST"
		},		
		"evmVersion": "$evmVersion",
		"libraries": {},
		"metadata": {
			"bytecodeHash": "ipfs"
		},
		"optimizer": {
			"enabled": $optimizerEnabled,
			"runs": 200
		},
		"remappings": []
	},
	"sources": $(cat /tmp/sources.json),
	"version": 1
}
EOF

echo "Created metadata.json"
[[ -n "$entryFile" ]] || echo "  WARNING: .settings.compilationTarget has to be set manually. The key is the path to the entrypoint file, and the value is the entrypoint contract."


########## FORMAT: FOR SOURCIFY /verify API
{
echo "{"
echo "\"metadata.json\": $(cat metadata.json | jq -Rs)" 
#for filename in ${filenames[@]}; do
# for filename in sources.sol; do
# 	echo ","
# 	echo "\"$filename\": $(cat $filename | jq -Rs)"
# done
echo "}"
} >flat-sources.json

cat >sourcify-request.json <<EOF
{
	"address": "${addr}",
	"chain": "${chainId}",
	"files": $(cat flat-sources.json),
	"chosenContract": "0"
}
EOF
# ^ chosenContract needs to be a number, not the contract name. It is the index of the contract in the list of contracts ... as generated on the server from the source.
# There seems to always only be 1 contract in the list, so it's always 0. (Even if the .sol defines multiple contracts.)
if true; then
	echo "Submitting sources to Sourcify (sending sourcify-request.json to /verify API)"
	curl -sS -d @$(dirname $input_file)/sourcify-request.json -H 'Content-Type: application/json' $SOURCIFY_SERVER/verify \
		>sourcify-response--verify.json \
		&& echo "SUCCESS!" \
		|| { cat sourcify-response--verify.json; echo; }
fi

########## FORMAT: FOR /verify/solc-json API - INCOMPLETE
# The format of "files" is not really documented and is wrong below; should be like
#   https://github.com/ethereum/sourcify/blob/3681ece90acc307cbd7a3a21e25b2f1752d06695/services/server/test/testcontracts/Storage/StorageJsonInput.json#L4
# as per
#   https://github.com/ethereum/sourcify/blob/staging/services/server/test/server.js#L719

# cat >solc-json-api.json <<EOF
# {
#   "address": "$addr",
#   "chain": "$chainId",
#   "files": { 
# 		"metadata.json": $(cat metadata.json | jq -Rs),
# 		"sources.sol": $(cat sources.sol | jq -Rs)
# 	},
#   "compilerVersion": "$(jq <$input_file -r '.result[0].CompilerVersion')",
#   "contractName": "$mainContract"
# }
# EOF
# if false; then
# 	set -x
# 	curl -sS -d @$(dirname $input_file)/solc-json-api.json -H 'Content-Type: application/json' https://sourcify.dev/server/verify/solc-json
# 	set +x
# fi

########## FORMAT: For "Import from Solidity JSON" UI on Sourcify. Uses session-based Sourcify APIs under the hood.
# Ref: https://docs.soliditylang.org/en/v0.8.20/using-the-compiler.html#input-description

# cat >solidity-input.json <<EOF
# {
# 	"language": "Solidity",
# 	"sources": $(cat /tmp/sources.json)
# }
# EOF
# echo "Created solidity-input.json to use with \"Import from Solidity JSON\" UI on Sourcify."
	
# if false; then
# 	# Does not work. Would need session management, CORS.
# 	cmd="curl -sS -F files=@${PWD}/solidity-input.json -F compilerVersion='$(jq <$input_file '.result[0].CompilerVersion')' https://sourcify.dev/server/session/input-solc-json" 
# 	echo "Submitting sources to Sourcify:  $cmd"
# 	ok=0
# 	for _ in 1 2 3 4 5; do
# 		$cmd >verification-result.json && { ok=1; break; }
# 	done
# 	[[ $ok != 0 ]] || exit 1
# 	[[ "$(jq <verification-result.json '.error == null')" == "true" ]] || { echo "Compilation error:"; cat verification-result.json; echo; exit 1; }

# 	verificationId=$(jq <verification-result.json -r ".contracts[] | select(.name == \"$mainContract\") | .verificationId")

# 	curl -sS 'https://sourcify.dev/server/session/verify-validated' -X POST -H 'Content-Type: application/json'--data-raw '{"contracts":[{"verificationId":"'$verificationId'","address":"0xEf5Cb745B8Cb5323Cd2Cb1625Bd8678099997812","chainId":"'$chainId'","creatorTxHash":""}]}'
# fi

# ############## FORMAT: verifysourcecode API
# # I'm getting loopy ... this is a Blockscout API, not Sourcify.
# set -x
# curl --location 'https://.../api	?module=contract&action=verifysourcecode' \
# --form "contractaddress=\"$addr\"" \
# --form sourceCode=@sources.sol \
# --form "contractname=\"${mainContract}\"" \
# --form 'codeformat="solidity-single-file"' \
# --form "compilerversion=\"v0.8.17+commit.8df45f5f\"" \
# --form "optimizationUsed=\"$(jq <$input_file -r '.result[0].OptimizationUsed')\"" \
# --form "runs=\"$(jq <$input_file -r '.result[0].OptimizationRuns')\"" \
# --form 'constructorArguements=""' \
# --form "evmversion=\"$(jq <$input_file -r '.result[0].EVMVersion')\"" \
# --form 'autodetectConstructorArguments="true"'
# set +x
