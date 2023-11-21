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
  last_mutate_round UINT63 NOT NULL
);

COMMIT;