# EVM ABIs

This directory contains a Go package with Ethereum-compatible smart contract
application binary interfaces (ABIs) and the Solidity sources.

These interfaces are stable, so we don't have to maintain/rebuild them much. We
currently use the [Remix web IDE](https://remix.ethereum.org/) to compile the
Solidity sources (e.g. `contracts/ERC20.sol`) into JSON files containing EVM
bytecode and ABI records (e.g. `contracts/artifacts/ERC20.json`). This is not
automated, so for other developers' convenience, check in the compiled JSON
files.

The Go code [embeds](https://pkg.go.dev/embed#hdr-Strings_and_Bytes) the JSON
files and parses them into go-ethereum
[ABI](https://pkg.go.dev/github.com/ethereum/go-ethereum@v1.10.26/accounts/abi#ABI)
structures.
