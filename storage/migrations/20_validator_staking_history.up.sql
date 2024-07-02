BEGIN;

CREATE TABLE chain.validator_staking_history
(
    id base64_ed25519_pubkey NOT NULL,
    epoch UINT63 NOT NULL,
    PRIMARY KEY (id, epoch),
    escrow_balance_active UINT_NUMERIC NOT NULL,
    escrow_balance_debonding UINT_NUMERIC NOT NULL,
    escrow_total_shares_active UINT_NUMERIC NOT NULL,
    escrow_total_shares_debonding UINT_NUMERIC NOT NULL,
    num_delegators UIINT63
);

-- todo: figure out if we still need this to track all validators ever.
ALTER TABLE chain.epochs
    ADD COLUMN validators base64_ed25519_pubkey[];

COMMIT;
