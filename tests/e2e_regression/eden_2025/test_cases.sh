#!/bin/bash

# This file holds the test cases for the e2e_regression tests.
#
# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.

source "$E2E_REGRESSION_DIR/common_test_cases.sh"

testCases=(
  "${commonTestCases[@]}"
  "${commonMainnetTestCases[@]}"
  "${commonSapphireTestCases[@]}"
  'block                                       /v1/consensus/blocks/24214831'
  'blocks_proposed_by                          /v1/consensus/blocks?proposed_by=oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'entity                                      /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'entity_nodes                                /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes'
  'node                                        /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes/6wbL5%2fOxvFGxi55o7AxcwKmfjXbXGC1hw4lfnEZxBXA='
  'bad_node                                    /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes/NOTANODE'
  'epoch                                       /v1/consensus/epochs/40345'
  'tx                                          /v1/consensus/transactions/b3bac3243ad44adcb8829ed17599efd05d9c6dff3477a085d2eaa4032ab73be3'
  'tx_failed                                   /v1/consensus/transactions/ad590f1452e19b3976fcf23fc050da6ed347abe476491256a56aed766e43e232'
  'validator                                   /v1/consensus/validators/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'validator_by_entity_id                      /v1/consensus/validators?id=9sAhd%2BWi6tG5nAr3LwXD0y9mUKLYqfAbS2%2B7SZdNHB4%3D'
  'validator_by_node_id                        /v1/consensus/validators?id=6wbL5%2FOxvFGxi55o7AxcwKmfjXbXGC1hw4lfnEZxBXA%3D'
  'validator_history                           /v1/consensus/validators/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/history'
  'sapphire_tx                                 /v1/sapphire/transactions/0x8075ce0d4ee6362ea127e20a10411662b173388cb792d4f22a04b594de03da80'
  'sapphire_failed_tx                          /v1/sapphire/transactions/0x4ea7e5688c0577a46efd3e193393bfb2968b589f1495b4350b006eae1e54e8ba'
  'sapphire_txs_by_related_account             /v1/sapphire/transactions?rel=0x6Cc4Fe9Ba145AbBc43227b3D4860FA31AFD225CB'
  'sapphire_txs_by_related_account_and_method  /v1/sapphire/transactions?rel=0x6Cc4Fe9Ba145AbBc43227b3D4860FA31AFD225CB&method=evm.Call_no_native'
  'sapphire_transfer_events_of_token           /v1/sapphire/events?evm_log_signature=ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef&contract_address=0x39d22B78A7651A76Ffbde2aaAB5FD92666Aca520'
  'sapphire_token                              /v1/sapphire/evm_tokens/0x39d22B78A7651A76Ffbde2aaAB5FD92666Aca520'
  'sapphire_token_holders                      /v1/sapphire/evm_tokens/0x39d22B78A7651A76Ffbde2aaAB5FD92666Aca520/holders'
  'sapphire_token_by_name                      /v1/sapphire/evm_tokens?name=USD'
  'sapphire_token_by_multiple_names            /v1/sapphire/evm_tokens?name=Token,Ocean'
  'sapphire_account_with_rose                  /v1/sapphire/accounts/0xCCD706B1c30d1c6F46F74885C3B13e3Ea28ce8AF'
  'sapphire_account_with_evm_token             /v1/sapphire/accounts/0x6Cc4Fe9Ba145AbBc43227b3D4860FA31AFD225CB'
  'sapphire_token_configured_addresses         /v1/sapphire/evm_tokens/0xA14167756d9F86Aed12b472C29B257BBdD9974C2'
  'sapphire_account_delegations                /v1/sapphire/accounts/oasis1qz83nq7a9cudtxnmmxxteqkeansfcsldjy3kcgqk/delegations'
  'sapphire_account_debonding_delegations      /v1/sapphire/accounts/oasis1qz83nq7a9cudtxnmmxxteqkeansfcsldjy3kcgqk/debonding_delegations'
)
