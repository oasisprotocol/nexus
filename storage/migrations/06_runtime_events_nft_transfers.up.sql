BEGIN;

CREATE INDEX ix_runtime_events_nft_transfers
    ON chain.runtime_events (runtime, (body ->> 'address'), (body -> 'topics' ->> 3), round)
    WHERE
        type = 'evm.log' AND
        evm_log_signature = '\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef' AND
        jsonb_array_length(body -> 'topics') = 4;

COMMIT;
