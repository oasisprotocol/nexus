#!/bin/bash

# This file holds the test cases for the e2e_regression tests.
#
# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.

source "$E2E_REGRESSION_DIR/common_test_cases.sh"

testCases=(
  "${commonTestCases[@]}"
  'block                              /v1/consensus/blocks/16818000'
  'blocks_proposed_by                 /v1/consensus/blocks?proposed_by=oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'entity                             /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'entity_nodes                       /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes'
  'node                               /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes/6wbL5%2fOxvFGxi55o7AxcwKmfjXbXGC1hw4lfnEZxBXA='
  'bad_node                           /v1/consensus/entities/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/nodes/NOTANODE'
  'epoch                              /v1/consensus/epochs/28017'
  'tx                                 /v1/consensus/transactions/142d43e5194b738ab2223f8d0b42326fab06edd714a8cefc59a078b89b5de057'
  'validator                          /v1/consensus/validators/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm'
  'validator_history                  /v1/consensus/validators/oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm/history'
  'emerald_tx                         /v1/emerald/transactions/ec1173a69272c67f126f18012019d19cd25199e831f9417b6206fb7844406f9d'
  'emerald_failed_tx                  /v1/emerald/transactions/35fdc8261dd81be8187c858aa9a623085494baf0565d414f48562a856147c093'
  'emerald_events_by_nft              /v1/emerald/events?contract_address=oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx&nft_id=1'
  'emerald_token                      /v1/emerald/evm_tokens/oasis1qpgcp5jzlgk4hcenaj2x82rqk8rrve2keyuc8aaf'
  'emerald_token_eth                  /v1/emerald/evm_tokens/0x21C718C22D52d0F3a789b752D4c2fD5908a8A733'
  'emerald_token_holders              /v1/emerald/evm_tokens/oasis1qpgcp5jzlgk4hcenaj2x82rqk8rrve2keyuc8aaf/holders'
  'emerald_token_nfts                 /v1/emerald/evm_tokens/oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx/nfts'
  'emerald_token_nfts_eth             /v1/emerald/evm_tokens/0x1108A83b867c8b720fEa7261AE7A64DAB17B4159/nfts'
  'emerald_token_nft                  /v1/emerald/evm_tokens/oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx/nfts/2'
  'emerald_account_with_rose          /v1/emerald/accounts/oasis1qrt0sv2s2x2lkt9e7kmr2mzxgme8m0pzauwztprl'
  'emerald_account_with_evm_token     /v1/emerald/accounts/oasis1qpl38f2a8m55ylha8ysdn03ya0mgrftkasmy8wyv'
  'emerald_account_with_evm_token_eth /v1/emerald/accounts/0x8d82559A91929DD72682f5C6E2EEA3905B6c2A18'
  'emerald_account_nfts               /v1/emerald/accounts/oasis1qzlqgyqp2fjla8r6rf5k3dd0k0qada9n5vyu4h3l/nfts'
  'emerald_account_nfts_eth           /v1/emerald/accounts/A3BD5b36659781AF4729c95c19003924ca3EF966/nfts'
  'emerald_account_nfts_token         /v1/emerald/accounts/oasis1qzlqgyqp2fjla8r6rf5k3dd0k0qada9n5vyu4h3l/nfts?token_address=oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx'
)
