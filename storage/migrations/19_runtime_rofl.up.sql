BEGIN;

CREATE DOMAIN public.rofl_app_id TEXT CHECK(length(VALUE) = 45 AND VALUE ~ '^rofl1');

CREATE TABLE chain.rofl_apps
(
  runtime  runtime NOT NULL,
  id rofl_app_id NOT NULL,
  PRIMARY KEY (runtime, id),

  admin oasis_addr,
  stake UINT_NUMERIC,

  policy JSONB,
  sek TEXT,
  metadata JSONB, -- arbitrary key/value pairs.
  -- metadata_name TEXT, -- Name, extracted from metadata key `net.oasis.rofl.name`. -- Added in 26_runtime_rofl_metadata_name.up.sql.
  secrets JSONB, -- arbitrary key/value pairs.

  -- num_transactions UINT63 NOT NULL, -- Added in 27_runtime_rofl_num_transactions.up.sql.
  -- created_at_round UINT63 NOT NULL, -- Added in 43_runtime_rofl_app_created_at.up.sql.

  removed BOOLEAN NOT NULL DEFAULT FALSE,

  -- Fields for analyzer tracking.
  last_queued_round UINT63 NOT NULL,
  last_processed_round UINT63
);
CREATE INDEX ix_rofl_apps_stale ON chain.rofl_apps (runtime, id) WHERE last_processed_round IS NULL OR last_queued_round > last_processed_round;
-- On the API endpoint we only ever return apps that have been processed.
CREATE INDEX ix_rofl_apps_processed ON chain.rofl_apps (runtime, id) WHERE last_processed_round IS NOT NULL;
-- CREATE INDEX ix_rofl_apps_metadata_name ON chain.rofl_apps USING GIN (metadata_name gin_trgm_ops); -- Added in 29_runtime_rofl_metadata_name_partial.up.sql.
-- CREATE INDEX ix_rofl_apps_admin ON chain.rofl_apps (runtime, admin); -- Added in 37_rofl_admin_idx.up.sql.

CREATE TABLE chain.rofl_instances
(
  runtime  runtime NOT NULL,
  app_id rofl_app_id NOT NULL,
  FOREIGN KEY (runtime, app_id) REFERENCES chain.rofl_apps(runtime, id), -- DEFERRABLE INITIALLY DEFERRED, Added in 25_runtime_roflmarket.up.sql.

  rak TEXT NOT NULL,
  PRIMARY KEY (runtime, app_id, rak),

  endorsing_node_id TEXT NOT NULL,
  endorsing_entity_id TEXT,
  rek TEXT NOT NULL,
  expiration_epoch UINT63 NOT NULL,
  extra_keys TEXT[],

  -- metadata JSONB, -- Added in 42_runtime_rofl_instances_metadata.up.sql.

  -- Fields for rofl instance transactions analyzer progress tracking.
  registration_round UINT63 NOT NULL,
  last_processed_round UINT63 NOT NULL
);
-- Support fetching all instances for a given app, sorted by expiration epoch.
CREATE INDEX ix_rofl_instances_runtime_app_id_expiration_epoch ON chain.rofl_instances (runtime, app_id, expiration_epoch);
-- Index for fetching instances that need to be processed - either active or have not yet been processed.
CREATE INDEX ix_rofl_instances_runtime_expiration_epoch_last_processed_round ON chain.rofl_instances (runtime, expiration_epoch) WHERE registration_round = last_processed_round;

-- Transactions relating to ROFL apps.
CREATE TABLE chain.rofl_related_transactions
(
  runtime runtime NOT NULL,
  app_id rofl_app_id NOT NULL,

  tx_round        UINT63 NOT NULL,
  tx_index        UINT31 NOT NULL,

  method TEXT,
  likely_native_transfer BOOLEAN NOT NULL DEFAULT FALSE,

  FOREIGN KEY (runtime, tx_round, tx_index) REFERENCES chain.runtime_transactions(runtime, round, tx_index) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX ix_rofl_related_transactions_app_id_round_index ON chain.rofl_related_transactions (runtime, app_id, tx_round, tx_index);
CREATE INDEX ix_rofl_related_transactions_app_id_method_round_index ON chain.rofl_related_transactions (runtime, app_id, method, tx_round, tx_index);
CREATE INDEX ix_rofl_related_transactions_app_id_likely_native_transfer_round_index ON chain.rofl_related_transactions (runtime, app_id, likely_native_transfer, tx_round, tx_index);
CREATE INDEX ix_rofl_related_transactions_app_id_method_likely_native_transfer_round_index ON chain.rofl_related_transactions (runtime, app_id, method, likely_native_transfer, tx_round, tx_index);

-- Transactions submitted by ROFL instances.
CREATE TABLE chain.rofl_instance_transactions
(
  runtime runtime NOT NULL,
  app_id rofl_app_id NOT NULL,
  rak TEXT NOT NULL,

  tx_round UINT63 NOT NULL,
  tx_index UINT31 NOT NULL,

  -- PRIMARY KEY (runtime, app_id, rak, tx_round, tx_index), -- Added in 21_rofl_related_remove_register.up.sql

  method TEXT,
  likely_native_transfer BOOLEAN NOT NULL DEFAULT FALSE,

  FOREIGN KEY (runtime, tx_round, tx_index) REFERENCES chain.runtime_transactions(runtime, round, tx_index) DEFERRABLE INITIALLY DEFERRED
);
-- Fetching per-app trannsactions.
CREATE INDEX ix_rofl_instance_transactions_runtime_app_id_round_index ON chain.rofl_instance_transactions (runtime, app_id, tx_round, tx_index);
CREATE INDEX ix_rofl_instance_transactions_runtime_app_id_method_round_index ON chain.rofl_instance_transactions (runtime, app_id, method, tx_round, tx_index);
CREATE INDEX ix_rofl_instance_transactions_runtime_app_id_likely_native_transfer_round_index ON chain.rofl_instance_transactions (runtime, app_id, likely_native_transfer, tx_round, tx_index);
CREATE INDEX ix_rofl_instance_transactions_runtime_app_id_method_likely_native_transfer_round_index ON chain.rofl_instance_transactions (runtime, app_id, method, likely_native_transfer, tx_round, tx_index);

-- Fetching per-instance transactions.
CREATE INDEX ix_rofl_instance_transactions_app_id_rak_round_index ON chain.rofl_instance_transactions (runtime, app_id, rak, tx_round, tx_index); -- Removed in 21_rofl_related_remove_register.up.sql
CREATE INDEX ix_rofl_instance_transactions_app_id_rak_method_round_index ON chain.rofl_instance_transactions (runtime, app_id, rak, method, tx_round, tx_index);
CREATE INDEX ix_rofl_instance_transactions_app_id_rak_likely_native_transfer_round_index ON chain.rofl_instance_transactions (runtime, app_id, rak, likely_native_transfer, tx_round, tx_index);
CREATE INDEX ix_rofl_instance_transactions_app_id_rak_method_likely_native_transfer_round_index ON chain.rofl_instance_transactions (runtime, app_id, rak, method, likely_native_transfer, tx_round, tx_index);

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
