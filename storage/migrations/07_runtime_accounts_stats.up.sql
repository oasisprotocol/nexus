BEGIN;

DROP INDEX IF EXISTS chain.ix_runtime_transfers_sender;
DROP INDEX IF EXISTS chain.ix_runtime_transfers_receiver;

ALTER TABLE chain.runtime_accounts
    ADD COLUMN total_sent UINT_NUMERIC NOT NULL DEFAULT 0,
    ADD COLUMN total_received UINT_NUMERIC NOT NULL DEFAULT 0;

WITH agg AS (
    SELECT runtime, sender, sum(amount) AS total_sent
    FROM chain.runtime_transfers
    WHERE sender IS NOT NULL
    GROUP BY 1, 2
)
INSERT INTO chain.runtime_accounts as accts (runtime, address, total_sent)
    SELECT runtime, sender, total_sent FROM agg
    ON CONFLICT (runtime, address) DO UPDATE
        SET total_sent = EXCLUDED.total_sent;

WITH agg AS (
    SELECT runtime, receiver, sum(amount) AS total_received
    FROM chain.runtime_transfers
    WHERE receiver IS NOT NULL
    GROUP BY 1, 2
)
INSERT INTO chain.runtime_accounts as accts (runtime, address, total_received)
    SELECT runtime, receiver, total_received FROM agg
    ON CONFLICT (runtime, address) DO UPDATE
        SET total_received = EXCLUDED.total_received;

COMMIT;
