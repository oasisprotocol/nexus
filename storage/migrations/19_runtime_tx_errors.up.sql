BEGIN;

ALTER TABLE chain.runtime_transactions
    -- The unparsed transaction error message. The "parsed" version will be 
    -- identical in the majority of cases. One notable exception are txs that
    -- were reverted inside the EVM; for those, the raw msg is abi-encoded.
    ADD COLUMN error_message_raw TEXT;

COMMIT;
