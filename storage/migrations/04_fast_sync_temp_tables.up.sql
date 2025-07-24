BEGIN;

-- Tables in this namespace are intended only for use during fast-sync.
--
-- They contain append-only data that would normally (during slow-sync) be
-- inserted into existing tables, but doing so during fast-sync would
-- cause too much DB lock contention.
--
-- NOTE: The data in these tables is NOT COMPLETE in that it is never guaranteed
--       to cover the full set of heights indexed so far. It should be used to update
--       existing tables at the end of fast sync, not to overwrite them.
CREATE SCHEMA IF NOT EXISTS todo_updates;

CREATE TABLE todo_updates.epochs( -- Tracks updates to chain.epochs(first_height,last_height)
  epoch UINT63,
  height UINT63
);
CREATE TABLE todo_updates.evm_token_balances( -- Tracks updates to analysis.evm_token_balances(last_mutate_round)
  runtime runtime NOT NULL,
  token_address oasis_addr NOT NULL,
  account_address oasis_addr NOT NULL,
  last_mutate_round UINT63 NOT NULL
);
CREATE TABLE todo_updates.evm_tokens( -- Tracks updates to chain.evm_tokens(last_mutate_round)
  runtime runtime NOT NULL,
  token_address oasis_addr NOT NULL,
  total_supply NUMERIC(1000,0) NOT NULL DEFAULT 0,
  num_transfers UINT63 NOT NULL DEFAULT 0,
  -- Tokens that likely emit no events and were only added via `additional_evm_token_addresses` will have this set to TRUE.
  -- This signals that num_transfers/num_holders values cannot be obtained.
  -- likely_no_events BOOLEAN NOT NULL DEFAULT FALSE, -- Added in 46_runtime_evm_tokens_no_events.up.sql.
  last_mutate_round UINT63 NOT NULL
);
-- Added in 09_fast_sync_temp_transaction_status_updates.up.sql.
-- CREATE TABLE todo_updates.transaction_status_updates( -- Tracks transaction status updates for consensus-accounts transactions.
--   runtime runtime NOT NULL,
--   round UINT63 NOT NULL,
--   method TEXT NOT NULL,
--   sender oasis_addr NOT NULL,
--   nonce UINT63 NOT NULL,

--   success BOOLEAN NOT NULL,
--   error_module TEXT,
--   error_code UINT63,
--   error_message TEXT
-- );

COMMIT;
