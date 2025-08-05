BEGIN;

CREATE TABLE todo_updates.block_signers( -- Tracks signers for blocks processed during fast-sync.
  block_height UINT63 NOT NULL,
  entity_ids TEXT[] NOT NULL
);

COMMIT;
