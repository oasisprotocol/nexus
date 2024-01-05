BEGIN;

CREATE TABLE chain.roothash_messages (
    runtime runtime NOT NULL,
    round UINT63 NOT NULL,
    message_index UINT31 NOT NULL,
    PRIMARY KEY (runtime, round, message_index),
    type TEXT,
    body JSONB,
    error_module TEXT,
    error_code UINT31,
    result BYTEA,
    related_accounts oasis_addr[]
);

COMMIT;
