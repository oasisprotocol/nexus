BEGIN;

-- This was missed in 19_runtime_rofl.up.sql.
ALTER TABLE chain.rofl_instances
  DROP CONSTRAINT rofl_instances_runtime_app_id_fkey;

ALTER TABLE chain.rofl_instances
  ADD CONSTRAINT rofl_instances_runtime_app_id_fkey
  FOREIGN KEY (runtime, app_id)
  REFERENCES chain.rofl_apps(runtime, id)
  DEFERRABLE INITIALLY DEFERRED;


CREATE TABLE chain.roflmarket_providers (
  runtime runtime NOT NULL,
  address oasis_addr NOT NULL,
  PRIMARY KEY (runtime, address),

  nodes TEXT[],
  scheduler rofl_app_id,
  payment_address  JSONB,
  metadata JSONB, -- arbitrary key/value pairs.

  stake UINT_NUMERIC,
  offers_next_id BYTEA,
  offers_count UINT63 NOT NULL DEFAULT 0,
  instances_next_id BYTEA,
  instances_count UINT63 NOT NULL DEFAULT 0,

  created_at TIMESTAMP WITH TIME ZONE,
  updated_at TIMESTAMP WITH TIME ZONE,

  removed BOOLEAN NOT NULL DEFAULT FALSE,

  -- Fields for analyzer tracking.
  last_queued_round UINT63 NOT NULL,
  last_processed_round UINT63
);

CREATE TABLE chain.roflmarket_offers (
  runtime runtime NOT NULL,
  id BYTEA NOT NULL,
  PRIMARY KEY (runtime, id), -- Removed in 40_runtime_roflmarket_constraints_fix.up.sql.
  -- PRIMARY KEY (runtime, provider, id), -- Added in 40_runtime_roflmarket_constraints_fix.up.sql.

  provider oasis_addr NOT NULL,
  FOREIGN KEY (runtime, provider) REFERENCES chain.roflmarket_providers(runtime, address) DEFERRABLE INITIALLY DEFERRED,

  resources JSONB,
  payment JSONB,
  capacity UINT63,
  metadata JSONB, -- arbitrary key/value pairs.

  removed BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX ix_roflmarket_offers_provider ON chain.roflmarket_offers (runtime, provider); -- Removed in 40_runtime_roflmarket_constraints_fix.up.sql.

CREATE TABLE chain.roflmarket_instances (
  runtime runtime NOT NULL,
  id BYTEA NOT NULL,
  PRIMARY KEY (runtime, id), -- Removed in 40_runtime_roflmarket_constraints_fix.up.sql.
  -- PRIMARY KEY (runtime, provider, id), -- Added in 40_runtime_roflmarket_constraints_fix.up.sql.

  provider oasis_addr NOT NULL,
  FOREIGN KEY (runtime, provider) REFERENCES chain.roflmarket_providers(runtime, address) DEFERRABLE INITIALLY DEFERRED,

  offer_id BYTEA NOT NULL,
  FOREIGN KEY (runtime, offer_id) REFERENCES chain.roflmarket_offers(runtime, id) DEFERRABLE INITIALLY DEFERRED, -- Removed in 40_runtime_roflmarket_constraints_fix.up.sql.
  -- FOREIGN KEY (runtime, provider, offer_id) REFERENCES chain.roflmarket_offers(runtime, provider, id) DEFERRABLE INITIALLY DEFERRED, -- Added in 40_runtime_roflmarket_constraints_fix.up.sql.

  status SMALLINT CHECK  (status >= 0 AND status <= 255),
  creator oasis_addr,
  admin oasis_addr,
  node_id TEXT,
  metadata JSONB, -- arbitrary key/value pairs.
  resources JSONB,
  deployment JSONB,
  created_at TIMESTAMP WITH TIME ZONE,
  updated_at TIMESTAMP WITH TIME ZONE,

  paid_from TIMESTAMP WITH TIME ZONE,
  paid_until TIMESTAMP WITH TIME ZONE,
  payment JSONB,
  payment_address BYTEA,
  refund_data BYTEA,

  cmd_next_id BYTEA,
  cmd_count UINT63,

  cmds JSONB,

  removed BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX ix_roflmarket_instances_provider ON chain.roflmarket_instances (runtime, provider); -- Removed in 40_runtime_roflmarket_constraints_fix.up.sql.
-- CREATE INDEX ix_roflmarket_instances_admin ON chain.roflmarket_instances (runtime, admin); -- Added in 38_runtime_roflmarket_instances_admin.up.sql.

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
