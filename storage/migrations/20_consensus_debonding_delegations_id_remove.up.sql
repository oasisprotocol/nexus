BEGIN;

-- Create the new debonding delegations table.
CREATE TABLE chain.debonding_delegations_new
(
  delegatee  oasis_addr NOT NULL REFERENCES chain.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  delegator  oasis_addr NOT NULL REFERENCES chain.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  debond_end UINT63 NOT NULL,
  shares     UINT_NUMERIC NOT NULL,
  PRIMARY KEY (delegatee, delegator, debond_end)
);

-- Insert data from the old table into the new table, merging shares on conflict.
-- Skip debonding delegations that already ended (old debonding delegations table was not pruned correctly in all cases).
DO $$
DECLARE
  current_epoch UINT63;
BEGIN
  SELECT id INTO current_epoch
  FROM chain.epochs
  ORDER BY id DESC
  LIMIT 1;

  INSERT INTO chain.debonding_delegations_new (delegatee, delegator, debond_end, shares)
  SELECT delegatee,
         delegator,
         debond_end,
         SUM(shares) as shares
  FROM chain.debonding_delegations
  WHERE debond_end > current_epoch
  GROUP BY delegatee, delegator, debond_end
  ON CONFLICT (delegatee, delegator, debond_end) DO UPDATE
  SET shares = chain.debonding_delegations_new.shares + EXCLUDED.shares;
END $$;

-- Drop the old table and rename the new table.
DROP TABLE chain.debonding_delegations;
ALTER TABLE chain.debonding_delegations_new RENAME TO debonding_delegations;

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;
GRANT SELECT ON ALL TABLES IN SCHEMA analysis TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA analysis TO PUBLIC;

COMMIT;
