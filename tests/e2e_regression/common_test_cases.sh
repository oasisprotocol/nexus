#!/bin/bash

# This file holds common test cases for the e2e_regression tests.
#
# Each test case is a pair of (name, SQL query or URL).
# For SQL queries, the regression-tested output is the result of the query against the indexer DB.
# For URLs, the regression-tested output is the full HTTP response.

commonTestCases=(
  ## Consensus.
  'db__allowances                     select * from chain.allowances order by beneficiary, owner'
  'db__debonding_delegations          select debond_end, delegator, delegatee, shares from chain.debonding_delegations order by debond_end, delegator, delegatee, shares' # column `id` is internal use only, and not stable
  'db__delegations                    select * from chain.delegations where shares != 0 order by delegatee, delegator'
  'db__epochs                         select id, start_height, end_height, ARRAY(SELECT unnest(validators) ORDER BY 1) AS sorted_validators from chain.epochs order by id'
  'db__entities                       select * from chain.entities order by id'
  'db__events                         select tx_block, tx_index, tx_hash, type, ARRAY(SELECT account_address FROM chain.events_related_accounts ra WHERE ra.tx_block = e.tx_block AND ra.type = e.type AND ra.event_index = e.event_index ORDER BY 1) AS sorted_related_accounts, body::text from chain.events e order by tx_block, tx_index, type, body::text'
  'db__nodes                          select id, entity_id, roles, expiration, voting_power from chain.nodes order by id'
  'db__runtime_nodes                  select rn.*, n.roles FROM chain.runtime_nodes rn LEFT JOIN chain.nodes n ON (rn.node_id = n.id) ORDER BY runtime_id, node_id'
  'db__validator_staking_history      select * from history.validators order by epoch, id'
  ## Runtimes.
  'db__account_related_txs            select * from chain.runtime_related_transactions order by runtime, tx_round, tx_index, account_address'
  'db__runtime_accounts               select * from chain.runtime_accounts order by runtime, address'
  'db__runtime_transfers              select * from chain.runtime_transfers order by runtime, round, sender, receiver, amount'
  'db__runtime_txs                    select runtime, round, tx_hash, "to", fee, gas_used, method, evm_fn_name, evm_fn_params, error_message, error_params from chain.runtime_transactions order by runtime, round, tx_index'
  'db__runtime_events                 select runtime, round, type, tx_hash, evm_log_name, evm_log_params, evm_log_signature from chain.runtime_events order by runtime, round, tx_index, type, body'
  'db__contract_gas_use               select c.runtime, contract_address, (SELECT gas_for_calling FROM chain.runtime_accounts ra WHERE (ra.runtime = c.runtime) AND (ra.address = c.contract_address)) AS gas_used, timestamp as created_at from chain.evm_contracts c left join chain.runtime_transactions rt on (c.creation_tx = rt.tx_hash) order by runtime, contract_address'
  # sdk_balances, evm_balances: Do not query zero balances; whether they are stored depends on indexing order and fast-sync.
  'db__sdk_balances                   select * from chain.runtime_sdk_balances where balance != 0 order by runtime, account_address'
  'db__evm_balances                   select * from chain.evm_token_balances where balance != 0 order by runtime, token_address, account_address'
  'db__evm_tokens                     select runtime, token_address, token_type, token_name, symbol, decimals, total_supply, num_transfers from chain.evm_tokens order by token_address'
  'db__evm_contracts                  select runtime, contract_address, creation_tx, md5(abi::text) as abi_md5 from chain.evm_contracts order by runtime, contract_address'
  # address preimages:
  'db__address_preimages              select * from chain.address_preimages order by address'

  'status                             /v1/'
  'spec                               /v1/spec/v1.yaml'
  'total_supply                       /v1/consensus/total_supply_raw'
  'circulating_supply                 /v1/consensus/circulating_supply_raw'
  'trailing_slash                     /v1/consensus/accounts/?limit=1'
  'accounts                           /v1/consensus/accounts'
  'accounts_extraneous_key            /v1/consensus/accounts?foo=bar'
  'blocks                             /v1/consensus/blocks'
  'bad_account                        /v1/consensus/accounts/oasis1aaaaaaa'
  # NOTE: entity-related tests are not stable long-term because their output is a combination of
  #       the blockchain at a given height (which is stable) and the _current_ metadata_registry state.
  #       We circumvent this by not fetching from metadata_registry at all, so the same metadata (= none) is always present for the test.
  'entities                           /v1/consensus/entities'
  'epochs                             /v1/consensus/epochs'
  'events                             /v1/consensus/events'
  'proposals                          /v1/consensus/proposals'
  'proposal                           /v1/consensus/proposals/2'
  'votes                              /v1/consensus/proposals/2/votes'
  'tx_volume                          /v1/consensus/stats/tx_volume'
  'window_size                        /v1/consensus/stats/tx_volume?window_size_seconds=300&window_step_seconds=300'
  'nonstandard_window_size            /v1/consensus/stats/tx_volume?window_size_seconds=301&window_step_seconds=300'
  'active_accounts                    /v1/consensus/stats/active_accounts'
  'active_accounts_window             /v1/consensus/stats/active_accounts?window_step_seconds=300'
  'active_accounts_emerald            /v1/emerald/stats/active_accounts'
  'txs                                /v1/consensus/transactions'
  'txs_by_method                      /v1/consensus/transactions?method=staking.Transfer'
  'validators                         /v1/consensus/validators?limit=200'
  'invalid_runtime                    /v1/invalid_runtime/transactions'
)

commonMainnetTestCases=(
  'account                            /v1/consensus/accounts/oasis1qp0302fv0gz858azasg663ax2epakk5fcssgza7j'
  'account_with_tx                    /v1/consensus/accounts/oasis1qpn83e8hm3gdhvpfv66xj3qsetkj3ulmkugmmxn3'
  'runtime-only_account               /v1/consensus/accounts/oasis1qphyxz5csvprhnn09r49nuyzl0jdw0wsj5xpvsg2'
  'delegations                        /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/delegations'
  'delegations_to                     /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/delegations_to'
  'debonding_delegations              /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/debonding_delegations'
  'debonding_delegations_to           /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/debonding_delegations_to'
  'txs_by_related_account             /v1/consensus/transactions?rel=oasis1qpn83e8hm3gdhvpfv66xj3qsetkj3ulmkugmmxn3'
  'txs_by_related_account_and_method  /v1/consensus/transactions?rel=oasis1qqwllxt8tvqxt8sryeg3r8ts0yulyu8frqr2kzg5&method=staking.Transfer'
  'txs_by_sender_and_method           /v1/consensus/transactions?sender=oasis1qr85nf4hnsw6crrf9l9ppzq5ucyynnzqqy300cyv&method=staking.Transfer'
)

commonEmeraldTestCases=(
  'emerald_blocks                     /v1/emerald/blocks'
  'emerald_txs                        /v1/emerald/transactions'
  'emerald_txs_by_method              /v1/emerald/transactions?method=consensus.Withdraw'
  'emerald_txs_native_transfers       /v1/emerald/transactions?method=native_transfers'
  'emerald_txs_evm_call_no_native     /v1/emerald/transactions?method=evm.Call_no_native'
  'emerald_txs_evm_call               /v1/emerald/transactions?method=evm.Call'
  'emerald_events                     /v1/emerald/events'
  'emerald_events_by_type             /v1/emerald/events?type=accounts.transfer'
  'emerald_tokens                     /v1/emerald/evm_tokens'
  'emerald_tokens_sort_market_cap     /v1/emerald/evm_tokens?sort_by=market_cap'
  'emerald_tokens_erc20               /v1/emerald/evm_tokens?type=ERC20'
  'emerald_tokens_erc721              /v1/emerald/evm_tokens?type=ERC721'
  'emerald_status                     /v1/emerald/status'
  'emerald_tx_volume                  /v1/emerald/stats/tx_volume'
  'emerald_contract_account           /v1/emerald/accounts/oasis1qz2rynvcmrkwd57v00298uc2vtzgatde3cjpy72f'
)

commonSapphireTestCases=(
  'sapphire_blocks                          /v1/sapphire/blocks'
  'sapphire_txs                             /v1/sapphire/transactions'
  'sapphire_txs_by_method                   /v1/sapphire/transactions?method=consensus.Withdraw'
  'sapphire_txs_native_transfers            /v1/sapphire/transactions?method=native_transfers'
  'sapphire_txs_evm_call                    /v1/sapphire/transactions?method=evm.Call'
  'sapphire_events                          /v1/sapphire/events'
  'sapphire_events_by_type                  /v1/sapphire/events?type=accounts.transfer'
  'sapphire_tokens                          /v1/sapphire/evm_tokens'
  'sapphire_tokens_sort_market_cap          /v1/sapphire/evm_tokens?sort_by=market_cap'
  'sapphire_status                          /v1/sapphire/status'
  'sapphire_tx_volume                       /v1/sapphire/stats/tx_volume'
  'sapphire_rofl_apps                       /v1/sapphire/rofl_apps'
  'sapphire_rofl_apps_sort_created_at       /v1/sapphire/rofl_apps?sort_by=created_at'
  'sapphire_rofl_apps_sort_created_at_desc  /v1/sapphire/rofl_apps?sort_by=created_at_desc'
  'sapphire_roflmarket_providers            /v1/sapphire/roflmarket_providers'
  'sapphire_roflmarket_instances            /v1/sapphire/roflmarket_instances'
)
