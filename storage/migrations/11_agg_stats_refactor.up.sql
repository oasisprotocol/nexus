-- Drop stats materialized views and replace with tables.
BEGIN;

-- Drop the old materialized views.
DROP MATERIALIZED VIEW IF EXISTS stats.daily_tx_volume;
DROP MATERIALIZED VIEW IF EXISTS stats.min5_tx_volume;

-- Create new tables.

-- min5_tx_volume stores the 5-minute tx volumes in 5-minute windows, per layer.
CREATE TABLE stats.min5_tx_volume
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  tx_volume uint63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);
-- daily_tx_volume stores the sliding window for the number of transactions per day per layer.
CREATE TABLE stats.daily_tx_volume
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  tx_volume uint63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);
-- Index for efficient query of the daily samples.
CREATE INDEX ix_stats_daily_tx_volume_daily_windows ON stats.daily_tx_volume (layer, window_end) WHERE ((window_end AT TIME ZONE 'UTC')::time = '00:00:00');

-- Grant read-only use for newly-created tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA stats TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA stats TO PUBLIC;

COMMIT;
