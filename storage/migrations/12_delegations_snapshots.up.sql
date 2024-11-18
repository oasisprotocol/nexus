BEGIN;

CREATE TABLE history.delegations_snapshots
(
  block UINT63 NOT NULL,
  delegatee oasis_addr NOT NULL,
  delegator oasis_addr NOT NULL, -- NULL in 06_escrow_history_delegator.up.sql
  shares    UINT_NUMERIC
);
CREATE INDEX ix_history_delegations_snapshots_block_delegator ON history.delegations_snapshots(block, delegator);

CREATE INDEX ix_history_escrow_events_epoch_delegator ON history.escrow_events(epoch, delegator);

COMMIT;
