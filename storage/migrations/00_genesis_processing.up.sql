BEGIN;

-- Keeps track of chains for which we've already processed the genesis data.
CREATE TABLE processed_geneses (
    chain_id TEXT NOT NULL,
    analyzer TEXT NOT NULL,
    processed_time TIMESTAMP WITH TIME ZONE NOT NULL,

    PRIMARY KEY (chain_id, analyzer)
);

COMMIT;
