BEGIN;

CREATE SCHEMA IF NOT EXISTS history;
GRANT USAGE ON SCHEMA history TO PUBLIC;

CREATE TABLE history.validators
(
    id base64_ed25519_pubkey NOT NULL,
    epoch UINT63 NOT NULL,
    PRIMARY KEY (id, epoch),
    escrow_balance_active UINT_NUMERIC NOT NULL,
    escrow_balance_debonding UINT_NUMERIC NOT NULL,
    escrow_total_shares_active UINT_NUMERIC NOT NULL,
    escrow_total_shares_debonding UINT_NUMERIC NOT NULL,
    num_delegators UINT63
);

ALTER TABLE chain.epochs
    ADD COLUMN validators base64_ed25519_pubkey[];

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA history TO PUBLIC;

COMMIT;
