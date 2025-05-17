BEGIN;

ALTER TABLE chain.rofl_apps
  ADD COLUMN num_transactions UINT63 NOT NULL DEFAULT 0;

-- Populate num_transactions from rofl related and instance transactions.
UPDATE chain.rofl_apps apps
  SET num_transactions = tx_counts.total
  FROM (
    SELECT runtime, app_id, COUNT(*) AS total
    FROM (
      SELECT runtime, app_id
      FROM chain.rofl_related_transactions

      UNION ALL

      SELECT runtime, app_id
      FROM chain.rofl_instance_transactions
    ) all_tx
    GROUP BY runtime, app_id
  ) tx_counts
  WHERE
    apps.runtime = tx_counts.runtime AND apps.id = tx_counts.app_id;

COMMIT;
