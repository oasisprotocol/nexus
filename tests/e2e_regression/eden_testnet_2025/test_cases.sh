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
  'block                                               /v1/consensus/blocks/25640984'
  # 'blocks_proposed_by                                /v1/consensus/blocks?proposed_by=oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm' # Enable once https://github.com/oasisprotocol/nexus/issues/795 is fixed.
  'entity                                              /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha'
  'entity_nodes                                        /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/nodes'
  'node                                                /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/nodes/qQEtJm4Dyd3k02BFU9V8n6kNnCPAzTvcbxWiPM3t7Xw='
  'bad_node                                            /v1/consensus/entities/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/nodes/NOTANODE'
  'epoch                                               /v1/consensus/epochs/42195'
  'tx                                                  /v1/consensus/transactions/5cfd9bbd204fd2f287e1be7b791905325f22837d46ebb80a93cfe874c1082c2c'
  'tx_failed                                           /v1/consensus/transactions/30400ff54d30f35983b149a7c95d577c95f89080b78c9cba54d21e5b0ec2b8c1'
  'validator                                           /v1/consensus/validators/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha'
  'validator_history                                   /v1/consensus/validators/oasis1qqv25adrld8jjquzxzg769689lgf9jxvwgjs8tha/history'
  'validator_node_account                              /v1/consensus/accounts/oasis1qzs0a640ax6qplzt5zg9tvw94qwr7670rvs6xyld'
  'sapphire_tx                                         /v1/sapphire/transactions/0xd82792e933ef7e843324db7bebe1f0930370e673656f541a2aebc791ead184ec'
  'sapphire_failed_tx                                  /v1/sapphire/transactions/0x8368ebf06e2498d5a927a5de81b7b89272693491b0f815a819d2491fc0230198'
  'sapphire_txs_by_related_account                     /v1/sapphire/transactions?rel=oasis1qp0j04msfhsdjrf5jpzr08v8yw9690ts9ycqv4j9'
  'sapphire_txs_by_related_account_and_method          /v1/sapphire/transactions?rel=oasis1qp0j04msfhsdjrf5jpzr08v8yw9690ts9ycqv4j9&method=consensus.Withdraw'
  'sapphire_transfer_events_of_token                   /v1/sapphire/events?evm_log_signature=ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef&contract_address=0xbAE528E422853AeBD630E8606e312aeA954FA5eB'
  'sapphire_token                                      /v1/sapphire/evm_tokens/0xbAE528E422853AeBD630E8606e312aeA954FA5eB'
  'sapphire_token_holders                              /v1/sapphire/evm_tokens/0xbAE528E422853AeBD630E8606e312aeA954FA5eB/holders'
  'sapphire_account_with_rose                          /v1/sapphire/accounts/0xeDA395666E56dd9E2Ef3Bdc76eee373b738640DD'
  'sapphire_account_with_evm_token                     /v1/sapphire/accounts/0xeDA395666E56dd9E2Ef3Bdc76eee373b738640DD'
  'sapphire_rofl_app                                   /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw'
  'sapphire_rofl_app_by_admin                          /v1/sapphire/rofl_apps?admin=oasis1qpwaggvmhwq5uk40clase3knt655nn2tdy39nz2f'
  'sapphire_rofl_app_by_name                           /v1/sapphire/rofl_apps?name=demo-rofl'
  'sapphire_rofl_app_by_multiple_names                 /v1/sapphire/rofl_apps?name=chatbot,demo'
  'sapphire_rofl_app_instances                         /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instances'
  'sapphire_rofl_app_instance                          /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instances/8EUGnz+hAqblEMWh+ZHKyZU7CSItm1wrJqK15dAjlfI='
  'sapphire_rofl_app_transactions                      /v1/sapphire/rofl_apps/rofl1qr5s5r9pkdkhmj7l5z4nuncjkc05p62m9uqa2svc/transactions'
  'sapphire_rofl_app_instance_transactions             /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instance_transactions'
  'sapphire_rofl_app_instance_transactions_method      /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instance_transactions?method=rofl.Register'
  'sapphire_rofl_app_per_instance_transactions         /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instances/8EUGnz+hAqblEMWh+ZHKyZU7CSItm1wrJqK15dAjlfI=/transactions'
  'sapphire_rofl_app_per_instance_transactions_method  /v1/sapphire/rofl_apps/rofl1qp55evqls4qg6cjw5fnlv4al9ptc0fsakvxvd9uw/instances/8EUGnz+hAqblEMWh+ZHKyZU7CSItm1wrJqK15dAjlfI=/transactions?method=rofl.Register'
  'sapphire_rofl_market_provider_offers                /v1/sapphire/roflmarket_providers/oasis1qp2ens0hsp7gh23wajxa4hpetkdek3swyyulyrmz/offers'
  'sapphire_rofl_market_provider_instances             /v1/sapphire/roflmarket_providers/oasis1qp2ens0hsp7gh23wajxa4hpetkdek3swyyulyrmz/instances'
)
