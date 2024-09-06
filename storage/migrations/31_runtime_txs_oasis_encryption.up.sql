BEGIN;

ALTER TABLE chain.runtime_transactions
    -- Encrypted data in encrypted Oasis-format transactions.
    ADD COLUMN oasis_encrypted_format call_format,
    ADD COLUMN oasis_encrypted_public_key BYTEA,
    ADD COLUMN oasis_encrypted_data_nonce BYTEA,
    ADD COLUMN oasis_encrypted_data_data BYTEA,
    ADD COLUMN oasis_encrypted_result_nonce BYTEA,
    ADD COLUMN oasis_encrypted_result_data BYTEA;

COMMIT;
