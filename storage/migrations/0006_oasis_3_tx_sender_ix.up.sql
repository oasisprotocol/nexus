-- Creates an index on transaction sender. This is a very common
-- query, and is unusably slow otherwise.
CREATE INDEX ix_transactions_sender ON oasis_3.transactions (sender);
