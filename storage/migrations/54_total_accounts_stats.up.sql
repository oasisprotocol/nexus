-- Add stats table for tracking total number of accounts per layer.

BEGIN;

-- total_accounts stores the daily snapshot of total accounts per layer.
CREATE TABLE stats.total_accounts
(
  layer           TEXT NOT NULL,
  window_end      TIMESTAMP WITH TIME ZONE NOT NULL,
  total_accounts  UINT63 NOT NULL,

  PRIMARY KEY (layer, window_end)
);

-- Grant read access.
GRANT SELECT ON stats.total_accounts TO PUBLIC;

COMMIT;
