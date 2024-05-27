BEGIN;

CREATE TABLE chain.validator_balance_history
(
    id base64_ed25519_pubkey NOT NULL,
    epoch UINT63 NOT NULL,
    PRIMARY KEY (id, epoch),
    escrow_balance_active UINT_NUMERIC NOT NULL,
    escrow_balance_debonding UINT_NUMERIC NOT NULL
);
CREATE INDEX ix_validator_balance_history_id_epoch ON chain.validator_balance_history (id, epoch);

ALTER TABLE chain.epochs
    ADD COLUMN validators base64_ed25519_pubkey[];

COMMIT;
