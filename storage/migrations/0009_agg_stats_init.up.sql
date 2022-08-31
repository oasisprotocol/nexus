-- Initialization of materialized views for aggregate statistics on the consensus layer.

BEGIN;

-- min5_tx_volume stores the transaction volume in 5 minute buckets
-- This can be used to estimate real time TPS.
CREATE MATERIALIZED VIEW min5_tx_volume AS
  SELECT
    date_trunc( 'hour', b.time ) AS hour,
    ( extract( minute FROM b.time )::int / 5 ) AS min_slot,
    COUNT(*) AS tx_volume
  FROM oasis_3.blocks AS b
    INNER JOIN oasis_3.transactions AS t ON b.height = t.block
  GROUP BY 1, 2;

-- daily_tx_volume stores the number of transactions per day
-- at the consensus layer.
CREATE MATERIALIZED VIEW daily_tx_volume AS
  SELECT
    date_trunc ( 'day', hour ) AS day,
    SUM(tx_volume) AS daily_tx_volume
  FROM min5_tx_volume
  GROUP BY 1;

COMMIT;
