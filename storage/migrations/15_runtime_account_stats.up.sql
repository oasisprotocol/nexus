BEGIN;

CREATE TABLE chain.runtime_accounts
(
    runtime runtime NOT NULL,
    address oasis_addr NOT NULL,
    PRIMARY KEY (runtime, address),

    num_txs UINT63 NOT NULL DEFAULT 0
);

-- Backfill chain.runtime_accounts
INSERT INTO chain.runtime_accounts (runtime, address, num_txs)
    SELECT runtime, account_address, COUNT(*) 
    FROM chain.runtime_related_transactions
    GROUP BY (runtime, account_address);

-- Grant others read-only use.
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
