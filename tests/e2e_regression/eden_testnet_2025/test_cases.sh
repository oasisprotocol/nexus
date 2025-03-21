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
  'block                              /v1/consensus/blocks/25326932'
  # 'blocks_proposed_by                 /v1/consensus/blocks?proposed_by=oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm' // Enable once https://github.com/oasisprotocol/nexus/issues/795 is fixed.
  'entity                             /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha'
  'entity_nodes                       /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/nodes'
  'node                               /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/nodes/qQEtJm4Dyd3k02BFU9V8n6kNnCPAzTvcbxWiPM3t7Xw='
  'bad_node                           /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/nodes/NOTANODE'
  'epoch                              /v1/consensus/epochs/42195'
  'tx                                 /v1/consensus/transactions/ddccddd3af42b50bccbd57b590ae6c324a54c0d8b6f5e0bc083c5d7c13683aa6'
  'tx_failed                          /v1/consensus/transactions/cb2126c7f6d7171d24af07697516d84cc8bed83e76784b81955d3c9075f354a5'
  'validator                          /v1/consensus/validators/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha'
  'validator_history                  /v1/consensus/validators/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/history'
  'sapphire_tx                        /v1/sapphire/transactions/0x10697239929a287947d2238f71070452643acc96eabac11b94dcba1e55dc2816'
  'sapphire_failed_tx                 /v1/sapphire/transactions/0xfe23f52c78c421753ebfdcfef46278059108ecea7b1c3a44161de1631480f329'
  'sapphire_transfer_events_of_token  /v1/sapphire/events?evm_log_signature=ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef&contract_address=0xbAE528E422853AeBD630E8606e312aeA954FA5eB'
  'sapphire_token                     /v1/sapphire/evm_tokens/0xbAE528E422853AeBD630E8606e312aeA954FA5eB'
  'sapphire_token_holders             /v1/sapphire/evm_tokens/0xbAE528E422853AeBD630E8606e312aeA954FA5eB/holders'
  'sapphire_account_with_rose         /v1/sapphire/accounts/0xeDA395666E56dd9E2Ef3Bdc76eee373b738640DD'
  'sapphire_account_with_evm_token    /v1/sapphire/accounts/0xeDA395666E56dd9E2Ef3Bdc76eee373b738640DD'
  'sapphire_rofl_app                  /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw'
  'sapphire_rofl_app_instances        /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instances'
  'sapphire_rofl_app_transactions     /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/transactions'
  # 'sapphire_rofl_app_instance_transactions' TODO..
)
