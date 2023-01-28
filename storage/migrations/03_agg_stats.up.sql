-- Initialization of materialized views for aggregate statistics on the consensus layer.

BEGIN;

-- Schema for aggregate statistics that are not tied to a specific chain "generation" (oasis_3, oasis_4, etc.). 
CREATE SCHEMA IF NOT EXISTS stats;
GRANT USAGE ON SCHEMA stats TO PUBLIC;

-- Rounds a given timestamp down to the nearest 5-minute "bucket" (e.g. 12:34:56 -> 12:30:00).
CREATE FUNCTION floor_5min (ts timestamptz) RETURNS timestamptz AS $$
    SELECT date_trunc('hour', $1) + date_part('minute', $1)::int / 5 * '5 minutes'::interval;
$$ LANGUAGE SQL IMMUTABLE;
GRANT EXECUTE ON FUNCTION floor_5min TO PUBLIC;


-- min5_tx_volume stores the consensus transaction volume in 5 minute buckets
-- This can be used to estimate real time TPS.
-- NOTE: This materialized view is NOT refreshed every 5 minutes due to computational cost.
CREATE MATERIALIZED VIEW stats.min5_tx_volume AS
  SELECT
    'consensus' AS layer,
    floor_5min(b.time) AS window_start,
    COUNT(*) AS tx_volume
  FROM oasis_3.blocks AS b
  JOIN oasis_3.transactions AS t ON b.height = t.block
  GROUP BY 2
  
  UNION ALL
  
  SELECT
    b.runtime::text AS layer,
    floor_5min(b.timestamp) AS window_start,
    COUNT(*) AS tx_volume
  FROM oasis_3.runtime_blocks AS b
  JOIN oasis_3.runtime_transactions AS t ON (b.round = t.round AND b.runtime = t.runtime)
  GROUP BY 1, 2;

-- daily_tx_volume stores the number of transactions per day
-- at the consensus layer.
CREATE MATERIALIZED VIEW stats.daily_tx_volume AS
  SELECT
    layer,
    date_trunc ( 'day', sub.window_start ) AS window_start,
    SUM(sub.tx_volume) AS tx_volume
  FROM stats.min5_tx_volume AS sub
  GROUP BY 1, 2;


-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA stats TO PUBLIC;

COMMIT;
