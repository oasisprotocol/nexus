-- Initialization of materialized views for aggregate statistics on the consensus layer.

BEGIN;

-- Rounds a given timestamp down to the nearest 5-minute "bucket" (e.g. 12:34:56 -> 12:30:00).
CREATE FUNCTION floor_5min (ts timestamptz) RETURNS timestamptz AS $$
    SELECT date_trunc('hour', $1) + date_part('minute', $1)::int / 5 * '5 minutes'::interval;
$$ LANGUAGE SQL IMMUTABLE;


-- min5_tx_volume stores the consensus transaction volume in 5 minute buckets
-- This can be used to estimate real time TPS.
-- NOTE: This materialized view is NOT refreshed every 5 minutes due to computational cost.
CREATE MATERIALIZED VIEW min5_tx_volume AS
  SELECT
    floor_5min(b.time) AS window_start,
    COUNT(*) AS tx_volume
  FROM oasis_3.blocks AS b
    INNER JOIN oasis_3.transactions AS t ON b.height = t.block
  GROUP BY 1;

-- daily_tx_volume stores the number of transactions per day
-- at the consensus layer.
CREATE MATERIALIZED VIEW daily_tx_volume AS
  SELECT
    date_trunc ( 'day', sub.window_start ) AS window_start,
    SUM(sub.tx_volume) AS tx_volume
  FROM min5_tx_volume AS sub
  GROUP BY 1;

COMMIT;
