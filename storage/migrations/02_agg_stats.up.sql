-- Initialization of materialized views and tables for aggregate statistics on the consensus layer.

BEGIN;

-- Schema for aggregate statistics.
CREATE SCHEMA IF NOT EXISTS stats;
GRANT USAGE ON SCHEMA stats TO PUBLIC;

-- min5_tx_volume stores the 5-minute tx volumes in 5-minute windows, per layer.
CREATE TABLE stats.min5_tx_volume
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  tx_volume       UINT63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);

-- daily_tx_volume stores the sliding window for the number of transactions per day per layer.
CREATE TABLE stats.daily_tx_volume
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  tx_volume       UINT63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);
-- Index for efficient query of the daily samples.
CREATE INDEX ix_stats_daily_tx_volume_daily_windows ON stats.daily_tx_volume (layer, window_end) WHERE ((window_end AT TIME ZONE 'UTC')::time = '00:00:00');

-- daily_active_accounts stores the sliding window for the number of unique accounts per day
-- that were involved in transactions.
CREATE TABLE stats.daily_active_accounts
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  active_accounts UINT63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);
-- Index for efficient query of the daily samples.
CREATE INDEX ix_stats_daily_active_accounts_daily_windows ON stats.daily_active_accounts (layer, window_end) WHERE ((window_end AT TIME ZONE 'UTC')::time = '00:00:00');

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA stats TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA stats TO PUBLIC;

COMMIT;
