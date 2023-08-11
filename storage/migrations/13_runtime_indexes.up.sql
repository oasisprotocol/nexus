BEGIN;

DROP INDEX chain.ix_runtime_related_transactions_address; -- This index is a prefix of an existing index and therefore unnecessary

DROP INDEX chain.ix_runtime_events_tx_hash;
DROP INDEX chain.ix_runtime_events_tx_eth_hash;
DROP INDEX chain.ix_runtime_events_evm_log_signature;

CREATE INDEX ix_runtime_events_tx_hash ON chain.runtime_events USING hash (tx_hash);
CREATE INDEX ix_runtime_events_tx_eth_hash ON chain.runtime_events USING hash (tx_eth_hash);
CREATE INDEX ix_runtime_events_evm_log_signature ON chain.runtime_events(runtime, evm_log_signature, round);

COMMIT;
