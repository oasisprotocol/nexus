BEGIN;

--`method` is a possible external API parameter; `block` lets us efficiently retrieve the most recent N txs with a given method.
CREATE INDEX IF NOT EXISTS ix_transactions_method_height ON chain.transactions (method, block);

DROP INDEX IF EXISTS chain.ix_transactions_sender;
--`sender` is a possible external API parameter; `block` lets us efficiently retrieve the most recent N txs with a given method.
CREATE INDEX IF NOT EXISTS ix_transactions_sender_block ON chain.transactions (sender, block);

COMMIT;
