#!/bin/bash

# This file holds the test cases for the e2e_regression tests.
#
# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.

source "$E2E_REGRESSION_DIR/common_test_cases.sh"

testCases=(
  "${commonTestCases[@]}"
  'block                          /v1/consensus/blocks/8049500'
  'entity                         /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As='
  'entity_nodes                   /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes'
  'node                           /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes/LuIdtuiEPLBJefXVieVruy4kf04jjp5CBJFWVes0ZuE='
  'bad_node                       /v1/consensus/entities/WazI78lMcmjyCH5+5RKkkfOTUR+XheHIohlqMu+a9As=/nodes/NOTANODE'
  'epoch                          /v1/consensus/epochs/13403'
  'tx                             /v1/consensus/transactions/f7a03e0912d355901ee794e5fec79a6b4c91363fc27d953596ee6de5c1492798'
  'validator                      /v1/consensus/validators/HPeLbzc88IoYEP0TC4nqSxfxdPCPjduLeJqFvmxFye8='
  'emerald_tx                     /v1/emerald/transactions/a6471a9c6f3307087586da9156f3c9876fbbaf4b23910cd9a2ac524a54d0aefe'
  'emerald_failed_tx              /v1/emerald/transactions/a7e76442c52a3cb81f719bde26c9a6179bd3415f96740d91a93ee8f205b45150'
  'emerald_token_nfts             /v1/emerald/evm_tokens/oasis1qqewaa87rnyshyqs7yutnnpzzetejecgeu005l8u/nfts'
  'emerald_token_nft              /v1/emerald/evm_tokens/oasis1qqewaa87rnyshyqs7yutnnpzzetejecgeu005l8u/nfts/227'
  'emerald_account_nfts           /v1/emerald/accounts/oasis1qq92lk7kpqmvllhjvhlc282zp6v2e2t2rqrwuq2u/nfts'
  'emerald_account_nfts_token     /v1/emerald/accounts/oasis1qq92lk7kpqmvllhjvhlc282zp6v2e2t2rqrwuq2u/nfts?token_address=oasis1qqewaa87rnyshyqs7yutnnpzzetejecgeu005l8u'
)
