-- Indexer state initialization for the Cobalt Upgrade.
-- https://docs.oasis.dev/general/mainnet/cobalt-upgrade/

-- Create Cobalt Upgrade Schema with `chain-id`.
CREATE SCHEMA oasis_2;

-- Block Data

CREATE TABLE oasis_2.blocks
(
  height     BIGINT PRIMARY KEY,
  block_hash BYTEA NOT NULL,
  time       TIMESTAMP NOT NULL,
  
  -- State Root Info
  namespace TEXT NOT NULL,
  version   BIGINT NOT NULL,
  type      TEXT NOT NULL,
  root_hash BYTEA NOT NULL,

  beacon     BYTEA,
  metadata   JSON,

  -- Checkpoint data.
  time_indexed TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE oasis_2.transactions
(
  block BIGINT NOT NULL REFERENCES oasis_2.blocks(height),

  txn_hash   BYTEA NOT NULL,
  txn_index  INTEGER,
  nonce      BIGINT NOT NULL,
  fee_amount NUMERIC,
  max_gas    BIGINT,
  method     TEXT NOT NULL,
  body       BYTEA,

  -- Error Fields
  -- This includes an encoding of no error.
  module  TEXT,
  code    INTEGER,
  message TEXT,

  -- We require a composite primary key since duplicate transactions can
  -- be included within blocks for this chain.
  PRIMARY KEY (block, txn_hash, txn_index),

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE oasis_2.events
(
  backend TEXT NOT NULL,
  type    TEXT NOT NULL,
  body    JSON,

  txn_block  BIGINT NOT NULL,
  txn_hash   BYTEA NOT NULL,
  txn_index  INTEGER,

  FOREIGN KEY (txn_block, txn_hash, txn_index) REFERENCES oasis_2.transactions(block, txn_hash, txn_index),

  -- Arbitrary additional data.
  extra_data JSON
);

-- Beacon Backend Data

CREATE TABLE oasis_2.epochs
(
  id           BIGINT PRIMARY KEY,
  start_height BIGINT NOT NULL,
  end_height   BIGINT,
  UNIQUE (start_height, end_height),

  -- Arbitrary additional data.
  extra_data JSON
);

-- Registry Backend Data
CREATE TABLE oasis_2.entities
(
  id      BYTEA PRIMARY KEY,
  address TEXT,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE oasis_2.nodes
(
  id         BYTEA PRIMARY KEY,
  entity_id  BYTEA NOT NULL REFERENCES oasis_2.entities(id),
  expiration BIGINT NOT NULL REFERENCES oasis_2.epochs(id),

  -- TLS Info
  tls_pubkey      BYTEA NOT NULL,
  tls_next_pubkey BYTEA,
  tls_addresses   TEXT[],

  -- P2P Info
  p2p_pubkey    BYTEA NOT NULL,
  p2p_addresses TEXT ARRAY,

  -- Consensus Info
  consensus_pubkey  BYTEA NOT NULL,
  consensus_address TEXT,

  -- VRF Info
  vrf_pubkey BYTEA,

  roles            INTEGER,
  software_version TEXT,
  is_validator     BOOLEAN,

  -- TODO: Track node status.

  -- Arbitrary additional data.
  extra_data JSON
);

-- Staking Data

CREATE TABLE oasis_2.accounts
(
  address TEXT PRIMARY KEY,
  
  -- General Account
  general_balance NUMERIC,
  nonce           BIGINT,

  -- Escrow Account
  escrow_balance_active         NUMERIC,
  escrow_total_shares_active    NUMERIC,
  escrow_balance_debonding      NUMERIC,
  escrow_total_shares_debonding NUMERIC,

  -- TODO: Track commission schedule and staking accumulator.

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE oasis_2.allowances
(
  allower   TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  allowee   TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  allowance NUMERIC,

  PRIMARY KEY (allower, allowee)
);

CREATE TABLE oasis_2.delegations
(
  delegatee TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  delegator TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  shares    NUMERIC NOT NULL,

  PRIMARY KEY (delegatee, delegator)
);

CREATE TABLE oasis_2.debonding_delegations
(
  delegatee  TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  delegator  TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  shares     NUMERIC NOT NULL,
  debond_end BIGINT NOT NULL,

  PRIMARY KEY (delegatee, delegator)
);

-- Scheduler Data

CREATE TABLE oasis_2.committee_members
(
  node      BYTEA NOT NULL REFERENCES oasis_2.nodes(id),
  valid_for BIGINT NOT NULL REFERENCES oasis_2.epochs(id),
  runtime   TEXT NOT NULL,
  kind      TEXT NOT NULL,
  role      TEXT NOT NULL,

  PRIMARY KEY (node, runtime, kind, role),

  -- Arbitrary additional data.
  extra_data JSON
);

-- Governance Data

CREATE TABLE oasis_2.proposals
(
  id            BIGINT PRIMARY KEY,
  submitter     TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  state         SMALLINT NOT NULL,
  deposit       NUMERIC NOT NULL,

  -- If this proposal is a new proposal.
  handler            TEXT,
  cp_target_version  TEXT,
  rhp_target_version TEXT,
  rcp_target_version TEXT,
  upgrade_epoch      BIGINT,

  -- If this proposal cancels an existing proposal.
  cancels BIGINT REFERENCES oasis_2.proposals(id),

  created_at    BIGINT NOT NULL,
  closes_at     BIGINT NOT NULL,
  invalid_votes BIGINT,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE oasis_2.votes
(
  proposal BIGINT NOT NULL REFERENCES oasis_2.proposals(id),
  voter    TEXT NOT NULL REFERENCES oasis_2.accounts(address),
  vote     TEXT,

  PRIMARY KEY (proposal, voter),

  -- Arbitrary additional data.
  extra_data JSON
);
