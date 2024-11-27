BEGIN;

-- Index used for fetching events emitted by a specific contract.
CREATE INDEX ix_runtime_events_evm_contract_events ON chain.runtime_events (runtime, (body ->> 'address'), evm_log_signature, round)
    WHERE
        type = 'evm.log';

COMMIT;
