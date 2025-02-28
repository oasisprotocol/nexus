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
  "${commonEmeraldTestCases[@]}"
  'block                          /v1/consensus/blocks/8049500'
  # 'blocks_proposed_by             /v1/consensus/blocks?proposed_by=oasis1qq0xmq7r0z9sdv02t5j9zs7en3n6574gtg8v9fyt' // Enable once https://github.com/oasisprotocol/nexus/issues/795 is fixed.
  'entity                         /v1/consensus/entities/oasis1qz0ea28d8p4xk8xztems60wq22f9pm2yyyd82tmt'
  'entity_nodes                   /v1/consensus/entities/oasis1qz0ea28d8p4xk8xztems60wq22f9pm2yyyd82tmt/nodes'
  'node                           /v1/consensus/entities/oasis1qz0ea28d8p4xk8xztems60wq22f9pm2yyyd82tmt/nodes/LuIdtuiEPLBJefXVieVruy4kf04jjp5CBJFWVes0ZuE='
  'bad_node                       /v1/consensus/entities/oasis1qz0ea28d8p4xk8xztems60wq22f9pm2yyyd82tmt/nodes/NOTANODE'
  'epoch                          /v1/consensus/epochs/13403'
  'tx                             /v1/consensus/transactions/f7a03e0912d355901ee794e5fec79a6b4c91363fc27d953596ee6de5c1492798'
  'validator                      /v1/consensus/validators/oasis1qr3w66akc8ud9a4zsgyjw2muvcfjgfszn5ycgc0a'
  'validator_history              /v1/consensus/validators/oasis1qq0xmq7r0z9sdv02t5j9zs7en3n6574gtg8v9fyt/history'
  'emerald_tx                     /v1/emerald/transactions/a6471a9c6f3307087586da9156f3c9876fbbaf4b23910cd9a2ac524a54d0aefe'
  'emerald_failed_tx              /v1/emerald/transactions/a7e76442c52a3cb81f719bde26c9a6179bd3415f96740d91a93ee8f205b45150'
  'emerald_token_nfts             /v1/emerald/evm_tokens/oasis1qqewaa87rnyshyqs7yutnnpzzetejecgeu005l8u/nfts'
  'emerald_token_nft              /v1/emerald/evm_tokens/oasis1qqewaa87rnyshyqs7yutnnpzzetejecgeu005l8u/nfts/227'
  'emerald_account_nfts           /v1/emerald/accounts/oasis1qq92lk7kpqmvllhjvhlc282zp6v2e2t2rqrwuq2u/nfts'
  'emerald_account_nfts_token     /v1/emerald/accounts/oasis1qq92lk7kpqmvllhjvhlc282zp6v2e2t2rqrwuq2u/nfts?token_address=oasis1qqewaa87rnyshyqs7yutnnpzzetejecgeu005l8u'
)
