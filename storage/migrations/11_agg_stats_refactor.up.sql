-- Drop stats materialized views and replace with tables.
-- Also migrate the data from the old materialized views to the new tables.

BEGIN;

-- First create new tables. The _new suffix is used to avoid name conflicts with the old materialized views,
-- and will be dropped at the end of the migration.

-- min5_tx_volume stores the 5-minute tx volumes in 5-minute windows, per layer.
CREATE TABLE stats.min5_tx_volume_new
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  tx_volume uint63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);
-- daily_tx_volume stores the sliding window for the number of transactions per day per layer.
CREATE TABLE stats.daily_tx_volume_new
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  tx_volume uint63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);
-- Index for efficient query of the daily samples.
CREATE INDEX ix_stats_daily_tx_volume_daily_windows ON stats.daily_tx_volume_new (layer, window_end) WHERE ((window_end AT TIME ZONE 'UTC')::time = '00:00:00');

-- Migrate the data from the old materialized views to the new tables.
INSERT INTO stats.min5_tx_volume_new
SELECT layer, window_start + INTERVAL '5 minute', tx_volume
FROM stats.min5_tx_volume;

INSERT INTO stats.daily_tx_volume_new
SELECT layer, window_start + INTERVAL '1 day', tx_volume
FROM stats.daily_tx_volume;

-- Drop the old materialized views.
DROP MATERIALIZED VIEW IF EXISTS stats.daily_tx_volume;
DROP MATERIALIZED VIEW IF EXISTS stats.min5_tx_volume;

-- Drop the last window for each of the daily_tx_volume layers, since that window can be incomplete.
WITH RankedRows AS (
    SELECT
        layer,
        window_end,
        ROW_NUMBER() OVER (PARTITION BY layer ORDER BY window_end DESC) AS rnum
    FROM
        stats.daily_tx_volume_new
)
DELETE FROM stats.daily_tx_volume_new
WHERE (layer, window_end) IN (SELECT layer, window_end FROM RankedRows WHERE rnum = 1);

-- Drop the last window for each of the min5_tx_volume layers, since that window can be incomplete.
WITH RankedRows AS (
    SELECT
        layer,
        window_end,
        ROW_NUMBER() OVER (PARTITION BY layer ORDER BY window_end DESC) AS rnum
    FROM
        stats.min5_tx_volume_new
)
DELETE FROM stats.min5_tx_volume_new
WHERE (layer, window_end) IN (SELECT layer, window_end FROM RankedRows WHERE rnum = 1);

-- Drop the _new suffixes from the new tables.
ALTER TABLE stats.min5_tx_volume_new
RENAME TO min5_tx_volume;
ALTER TABLE stats.daily_tx_volume_new
RENAME TO daily_tx_volume;

COMMIT;
