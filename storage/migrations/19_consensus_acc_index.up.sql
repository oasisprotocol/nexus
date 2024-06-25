BEGIN;

-- Enables sorting by general balance.
CREATE INDEX IF NOT EXISTS ix_chain_accounts_general_balance ON chain.accounts(general_balance);
-- TODO: maybe also by escrow.

COMMIT;
