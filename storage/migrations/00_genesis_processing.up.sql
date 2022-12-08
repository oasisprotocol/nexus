BEGIN;

-- Schema for data that is not tied to a specific chain.
CREATE SCHEMA IF NOT EXISTS multichain;
GRANT USAGE ON SCHEMA multichain TO PUBLIC;

-- Keeps track of chains for which we've already processed the genesis data.
CREATE TABLE multichain.processed_geneses (
    chain_id TEXT NOT NULL,  -- e.g. 'oasis_3'; corresponds to the name of the schema that contains the chain's data.
    analyzer TEXT NOT NULL,
    processed_time TIMESTAMP WITH TIME ZONE NOT NULL,

    PRIMARY KEY (chain_id, analyzer)
);

-- Grant others read-only use. This does NOT apply to future tables in the schema.
-- Likely not needed for this schema, but just in case.
GRANT SELECT ON ALL TABLES IN SCHEMA multichain TO PUBLIC;

COMMIT;
