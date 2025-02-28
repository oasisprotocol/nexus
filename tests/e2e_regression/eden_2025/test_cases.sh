#!/bin/bash

# This file holds the test cases for the e2e_regression tests.
#
# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.

source "$E2E_REGRESSION_DIR/common_test_cases.sh"

testCases=(
  "${commonTestCases[@]}"
  "${commonSapphireTestCases[@]}"
  'block                              /v1/consensus/blocks/7485561'
  # 'blocks_proposed_by                 /v1/consensus/blocks?proposed_by=oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm' // Enable once https://github.com/oasisprotocol/nexus/issues/795 is fixed.
  'entity                             /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'entity_nodes                       /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes'
  'node                               /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes/6wbL5%2fOxvFGxi55o7AxcwKmfjXbXGC1hw4lfnEZxBXA='
  'bad_node                           /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes/NOTANODE'
  'epoch                              /v1/consensus/epochs/37199'
  'tx                                 /v1/consensus/transactions/71bd66f0396d1818c246a103ee5a554e5e2ef15b1fbc970631098a28ea63e3fb'
  'validator                          /v1/consensus/validators/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'validator_history                  /v1/consensus/validators/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/history'
  'sapphire_tx                        /v1/sapphire/transactions/0x15fc545f59f4bd5294a17b0f0e0dc5c47a89df2547a751e9c282e94c04b5deb2'
  'sapphire_failed_tx                 /v1/sapphire/transactions/0x59107184b2d050ceb7b82600deed8a51bfc2c8ec7452e6e5de498d93134b9745'
  'sapphire_events_by_nft             /v1/sapphire/events?contract_address=0xc90a97BbeE6ea51CBddfb0AFC86486b1940BBF6b&nft_id=1'
  'sapphire_transfer_events_of_token  /v1/sapphire/events?evm_log_signature=ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef&contract_address=0xc90a97BbeE6ea51CBddfb0AFC86486b1940BBF6b'
  'sapphire_token                     /v1/sapphire/evm_tokens/0x6665a6Cae3F52959f0f653E3D04270D54e6f13d8'
  'sapphire_token_holders             /v1/sapphire/evm_tokens/0x6665a6Cae3F52959f0f653E3D04270D54e6f13d8/holders'
  'sapphire_token_nfts                /v1/sapphire/evm_tokens/0xc90a97BbeE6ea51CBddfb0AFC86486b1940BBF6b/nfts'
  'sapphire_token_nft                 /v1/sapphire/evm_tokens/0xc90a97BbeE6ea51CBddfb0AFC86486b1940BBF6b/nfts/2'
  'sapphire_account_with_rose         /v1/sapphire/accounts/0xD386C13aB6B0B7287b7C995C50d15EfBCEC241f0'
  'sapphire_account_with_evm_token    /v1/sapphire/accounts/0x05E0f460A790D2330ECEaBa9664f004D5Ea86A47'
  'sapphire_account_nfts              /v1/sapphire/accounts/0x05E0f460A790D2330ECEaBa9664f004D5Ea86A47/nfts'
  'sapphire_account_nfts_token        /v1/sapphire/accounts/0x05E0f460A790D2330ECEaBa9664f004D5Ea86A47/nfts?token_address=0x998633BDF6eE32A9CcA6c9A247F428596e8e65d8'
  # 'sapphire_rofl_app                  /v1/todo'
)
