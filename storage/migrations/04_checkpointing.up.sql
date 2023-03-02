BEGIN;

-- Heights at which the key tables have been "checkpointed" (i.e. copied from
-- table X to X_checkpoint) as part of `tests/genesis`.
CREATE TABLE chain.checkpointed_heights
(
  analyzer TEXT NOT NULL,
  height BIGINT PRIMARY KEY,
  checkpoint_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

COMMIT;
