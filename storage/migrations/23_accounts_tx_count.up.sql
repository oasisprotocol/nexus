BEGIN;

ALTER TABLE chain.accounts ADD COLUMN tx_count UINT63 NOT NULL DEFAULT 0;

UPDATE chain.accounts AS a
    SET tx_count = sub.tx_count
    FROM (
        SELECT
            account_address AS address,
            COUNT(*) AS tx_count
        FROM chain.accounts_related_transactions
        GROUP BY account_address
    ) AS sub
    WHERE a.address = sub.address;

COMMIT;
