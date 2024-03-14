#!/bin/bash

# This file holds the test cases for the e2e_regression tests.
#
# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.

source "$E2E_REGRESSION_DIR/common_test_cases.sh"

testCases=(
  "${commonTestCases[@]}"
  'block                          /v1/consensus/blocks/16818000'
  'entity                         /v1/consensus/entities/9sAhd+Wi6tG5nAr3LwXD0y9mUKLYqfAbS2+7SZdNHB4='
  'entity_nodes                   /v1/consensus/entities/9sAhd+Wi6tG5nAr3LwXD0y9mUKLYqfAbS2+7SZdNHB4=/nodes'
  'node                           /v1/consensus/entities/9sAhd+Wi6tG5nAr3LwXD0y9mUKLYqfAbS2+7SZdNHB4=/nodes/6wbL5%2fOxvFGxi55o7AxcwKmfjXbXGC1hw4lfnEZxBXA='
  'bad_node                       /v1/consensus/entities/9sAhd+Wi6tG5nAr3LwXD0y9mUKLYqfAbS2+7SZdNHB4=/nodes/NOTANODE'
  'epoch                          /v1/consensus/epochs/28017'
  'tx                             /v1/consensus/transactions/142d43e5194b738ab2223f8d0b42326fab06edd714a8cefc59a078b89b5de057'
  'validator                      /v1/consensus/validators/9sAhd+Wi6tG5nAr3LwXD0y9mUKLYqfAbS2+7SZdNHB4='
  'emerald_tx                     /v1/emerald/transactions/ec1173a69272c67f126f18012019d19cd25199e831f9417b6206fb7844406f9d'
  'emerald_failed_tx              /v1/emerald/transactions/35fdc8261dd81be8187c858aa9a623085494baf0565d414f48562a856147c093'
  'emerald_events_by_nft          /v1/emerald/events?contract_address=oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx&nft_id=1'
  'emerald_token_nfts             /v1/emerald/evm_tokens/oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx/nfts'
  'emerald_token_nft              /v1/emerald/evm_tokens/oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx/nfts/2'
  'emerald_account_nfts           /v1/emerald/accounts/oasis1qzlqgyqp2fjla8r6rf5k3dd0k0qada9n5vyu4h3l/nfts'
  'emerald_account_nfts_token     /v1/emerald/accounts/oasis1qzlqgyqp2fjla8r6rf5k3dd0k0qada9n5vyu4h3l/nfts?token_address=oasis1qz29t7nxkwfqgfk36uqqs9pzuzdt8zmrjud5mehx'
)
