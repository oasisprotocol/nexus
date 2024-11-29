BEGIN;

-- Index used for fetching all events of a specific type, of a specific contract (sorted by round).
-- For example listing all ERC20 Transfers of a specific token.
CREATE INDEX IF NOT EXISTS ix_runtime_events_evm_specific_contract_events ON chain.runtime_events (runtime, (body ->> 'address'), evm_log_signature, round)
    WHERE
        type = 'evm.log';

COMMIT;
