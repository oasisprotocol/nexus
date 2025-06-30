BEGIN;

ALTER TABLE chain.rofl_apps ADD COLUMN created_at_round UINT63 NOT NULL DEFAULT 0;

-- Populate created_at_round from rofl related and instance transactions.
UPDATE chain.rofl_apps apps
SET created_at_round = first_tx.first_round
FROM (
  SELECT
    txs.runtime,
    txs.app_id,
    MIN(txs.tx_round) AS first_round
  FROM (
    SELECT runtime, app_id, tx_round FROM chain.rofl_related_transactions
    UNION ALL
    SELECT runtime, app_id, tx_round FROM chain.rofl_instance_transactions
  ) txs
  GROUP BY txs.runtime, txs.app_id
) first_tx
WHERE
  apps.runtime = first_tx.runtime AND apps.id = first_tx.app_id;

COMMIT;
