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
  'db__epochs                         select * from chain.epochs order by id'
  'db__entities                       select * from chain.entities order by id'
  'db__events                         select tx_block, tx_index, tx_hash, type, ARRAY(SELECT unnest(related_accounts) ORDER BY 1) AS sorted_related_accounts, body::text from chain.events order by tx_block, tx_index, type, body::text'
  'db__nodes                          select id, entity_id, roles, expiration, voting_power from chain.nodes order by id'
  'db__runtime_nodes                  select rn.*, n.roles FROM chain.runtime_nodes rn LEFT JOIN chain.nodes n ON (rn.node_id = n.id) ORDER BY runtime_id, node_id'
  ## Runtimes.
  'db__account_related_txs            select * from chain.runtime_related_transactions order by runtime, tx_round, tx_index, account_address'
  'db__runtime_accounts               select * from chain.runtime_accounts order by runtime, address'
  'db__runtime_transfers              select * from chain.runtime_transfers order by runtime, round, sender, receiver'
  'db__runtime_txs                    select runtime, round, tx_hash, "to", fee, gas_used, method, evm_fn_name, evm_fn_params, error_message, error_params from chain.runtime_transactions order by runtime, round, tx_index'
  'db__runtime_events                 select runtime, round, type, tx_hash, evm_log_name, evm_log_params, evm_log_signature from chain.runtime_events order by runtime, round, tx_index, type, body'
  'db__contract_gas_use               select c.runtime, contract_address, (SELECT gas_for_calling FROM chain.runtime_accounts ra WHERE (ra.runtime = c.runtime) AND (ra.address = c.contract_address)) AS gas_used, timestamp as created_at from chain.evm_contracts c left join chain.runtime_transactions rt on (c.creation_tx = rt.tx_hash) order by runtime, contract_address'
  # sdk_balances, evm_balances: Do not query zero balances; whether they are stored depends on indexing order and fast-sync.
  'db__sdk_balances                   select * from chain.runtime_sdk_balances where balance != 0 order by runtime, account_address'
  'db__evm_balances                   select * from chain.evm_token_balances where balance != 0 order by runtime, token_address, account_address'
  'db__evm_tokens                     select runtime, token_address, token_type, token_name, symbol, decimals, total_supply, num_transfers from chain.evm_tokens order by token_address'
  'db__evm_contracts                  select runtime, contract_address, creation_tx, md5(abi::text) as abi_md5 from chain.evm_contracts order by runtime, contract_address'

  'status                             /v1/'
  'spec                               /v1/spec/v1.yaml'
  'trailing_slash                     /v1/consensus/accounts/?limit=1'
  'accounts                           /v1/consensus/accounts'
  'min_balance                        /v1/consensus/accounts?minTotalBalance=1000000'
  'big_int_balance                    /v1/consensus/accounts?minTotalBalance=999999999999999999999999999'
  'accounts_bad_big_int               /v1/consensus/accounts?minTotalBalance=NA'
  'accounts_extraneous_key            /v1/consensus/accounts?foo=bar'
  'blocks                             /v1/consensus/blocks'
  'bad_account                        /v1/consensus/accounts/oasis1aaaaaaa'
  'account                            /v1/consensus/accounts/oasis1qp0302fv0gz858azasg663ax2epakk5fcssgza7j'
  'account_with_tx                    /v1/consensus/accounts/oasis1qpn83e8hm3gdhvpfv66xj3qsetkj3ulmkugmmxn3'
  'runtime-only_account               /v1/consensus/accounts/oasis1qphyxz5csvprhnn09r49nuyzl0jdw0wsj5xpvsg2'
  'delegations                        /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/delegations'
  'delegations_to                     /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/delegations_to'
  'debonding_delegations              /v1/consensus/accounts/oasis1qpk366qvtjrfrthjp3xuej5mhvvtnkr8fy02hm2s/debonding_delegations'
  'debonding_delegations_to           /v1/consensus/accounts/oasis1qp0j5v5mkxk3eg4kxfdsk8tj6p22g4685qk76fw6/debonding_delegations_to'
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
  'validators                         /v1/consensus/validators'
  'emerald_blocks                     /v1/emerald/blocks'
  'emerald_txs                        /v1/emerald/transactions'
  'emerald_events                     /v1/emerald/events'
  'emerald_events_by_type             /v1/emerald/events?type=accounts.transfer'
  'emerald_tokens                     /v1/emerald/evm_tokens'
  'emerald_status                     /v1/emerald/status'
  'emerald_tx_volume                  /v1/emerald/stats/tx_volume'
  'emerald_contract_account           /v1/emerald/accounts/oasis1qz2rynvcmrkwd57v00298uc2vtzgatde3cjpy72f'
)
