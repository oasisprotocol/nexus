-- Indexer state initialization for test state.

BEGIN;

CREATE SCHEMA IF NOT EXISTS test;

-- Block Data

CREATE TABLE IF NOT EXISTS test.blocks
(
  height     BIGINT PRIMARY KEY,
  block_hash TEXT NOT NULL,
  time       TIMESTAMP NOT NULL,

  -- State Root Info
  namespace TEXT NOT NULL,
  version   BIGINT NOT NULL,
  type      TEXT NOT NULL,
  root_hash TEXT NOT NULL,

  beacon     BYTEA,
  metadata   JSON,

  -- Checkpoint data.
  time_indexed TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS test.transactions
(
  block BIGINT NOT NULL REFERENCES test.blocks(height),

  txn_hash   TEXT NOT NULL,
  txn_index  INTEGER,
  nonce      NUMERIC NOT NULL,
  fee_amount NUMERIC,
  max_gas    NUMERIC,
  method     TEXT NOT NULL,
  body       BYTEA,

  -- Error Fields
  -- This includes an encoding of no error.
  module  TEXT,
  code    BIGINT,
  message TEXT,

  -- We require a composite primary key since duplicate transactions can
  -- be included within blocks for this chain.
  PRIMARY KEY (block, txn_hash, txn_index),

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS test.events
(
  backend TEXT NOT NULL,
  type    TEXT NOT NULL,
  body    JSON,

  txn_block  BIGINT NOT NULL,
  txn_hash   TEXT NOT NULL,
  txn_index  INTEGER,

  FOREIGN KEY (txn_block, txn_hash, txn_index) REFERENCES test.transactions(block, txn_hash, txn_index),

  -- Arbitrary additional data.
  extra_data JSON
);

-- Beacon Backend Data

CREATE TABLE IF NOT EXISTS test.epochs
(
  id           BIGINT PRIMARY KEY,
  start_height BIGINT NOT NULL,
  end_height   BIGINT,
  UNIQUE (start_height, end_height),

  -- Arbitrary additional data.
  extra_data JSON
);

-- Registry Backend Data
CREATE TABLE IF NOT EXISTS test.entities
(
  id      TEXT PRIMARY KEY,
  address TEXT,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS test.nodes
(
  id         TEXT PRIMARY KEY,
  entity_id  TEXT NOT NULL REFERENCES test.entities(id),
  expiration BIGINT NOT NULL,

  -- TLS Info
  tls_pubkey      TEXT NOT NULL,
  tls_next_pubkey TEXT,
  tls_addresses   TEXT ARRAY,

  -- P2P Info
  p2p_pubkey    TEXT NOT NULL,
  p2p_addresses TEXT ARRAY,

  -- Consensus Info
  consensus_pubkey  TEXT NOT NULL,
  consensus_address TEXT,

  -- VRF Info
  vrf_pubkey TEXT,

  roles            TEXT,
  software_version TEXT,

  -- Voting power should only be nonzero for consensus validator nodes.
  voting_power     BIGINT DEFAULT 0,

  -- TODO: Track node status.

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS test.runtimes
(
  id           TEXT PRIMARY KEY,
  suspended    BOOLEAN NOT NULL DEFAULT false,
  kind         TEXT NOT NULL,
  tee_hardware TEXT NOT NULL,
  key_manager  TEXT,

  -- Arbitrary additional data.
  extra_data JSON
);

-- Staking Backend Data

CREATE TABLE IF NOT EXISTS test.accounts
(
  address TEXT PRIMARY KEY,

  -- General Account
  general_balance NUMERIC DEFAULT 0,
  nonce           BIGINT DEFAULT 0,

  -- Escrow Account
  escrow_balance_active         NUMERIC DEFAULT 0,
  escrow_total_shares_active    NUMERIC DEFAULT 0,
  escrow_balance_debonding      NUMERIC DEFAULT 0,
  escrow_total_shares_debonding NUMERIC DEFAULT 0,

  -- TODO: Track commission schedule and staking accumulator.

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS test.allowances
(
  owner       TEXT NOT NULL REFERENCES test.accounts(address),
  beneficiary TEXT NOT NULL REFERENCES test.accounts(address),
  allowance   NUMERIC,

  PRIMARY KEY (owner, beneficiary)
);

CREATE TABLE IF NOT EXISTS test.delegations
(
  delegatee TEXT NOT NULL REFERENCES test.accounts(address),
  delegator TEXT NOT NULL REFERENCES test.accounts(address),
  shares    NUMERIC NOT NULL,

  PRIMARY KEY (delegatee, delegator)
);

CREATE TABLE IF NOT EXISTS test.debonding_delegations
(
  delegatee  TEXT NOT NULL REFERENCES test.accounts(address),
  delegator  TEXT NOT NULL REFERENCES test.accounts(address),
  shares     NUMERIC NOT NULL,
  debond_end BIGINT NOT NULL
);

-- Scheduler Backend Data

CREATE TABLE IF NOT EXISTS test.committee_members
(
  node      TEXT NOT NULL,
  valid_for BIGINT NOT NULL,
  runtime   TEXT NOT NULL,
  kind      TEXT NOT NULL,
  role      TEXT NOT NULL,

  PRIMARY KEY (node, runtime, kind, role),

  -- Arbitrary additional data.
  extra_data JSON
);

-- Governance Backend Data

CREATE TABLE IF NOT EXISTS test.proposals
(
  id            BIGINT PRIMARY KEY,
  submitter     TEXT NOT NULL,
  state         TEXT NOT NULL DEFAULT 'active',
  executed      BOOLEAN NOT NULL DEFAULT false,
  deposit       NUMERIC NOT NULL,

  -- If this proposal is a new proposal.
  handler            TEXT,
  cp_target_version  TEXT,
  rhp_target_version TEXT,
  rcp_target_version TEXT,
  upgrade_epoch      BIGINT,

  -- If this proposal cancels an existing proposal.
  cancels BIGINT,

  created_at    BIGINT NOT NULL,
  closes_at     BIGINT NOT NULL,
  invalid_votes NUMERIC NOT NULL DEFAULT 0,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS test.votes
(
  proposal BIGINT NOT NULL REFERENCES test.proposals(id),
  voter    TEXT NOT NULL,
  vote     TEXT,

  PRIMARY KEY (proposal, voter),

  -- Arbitrary additional data.
  extra_data JSON
);

COMMIT;
