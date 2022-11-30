-- Indexer state initialization for the Damask Upgrade.
-- https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2022-04-11

BEGIN;

-- Create Damask Upgrade Schema with `chain-id`.
CREATE SCHEMA IF NOT EXISTS oasis_3;

-- Custom types
CREATE DOMAIN uint_numeric NUMERIC(1000,0) CHECK(VALUE >= 0);
CREATE DOMAIN uint63 BIGINT CHECK(VALUE >= 0);
CREATE DOMAIN uint31 INTEGER CHECK(VALUE >= 0);
CREATE DOMAIN hex64 TEXT CHECK(VALUE ~ '^[0-9a-f]{64}$');
-- base64(ed25519 public key); from https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/common/crypto/signature/signature.go#L90-L90
CREATE DOMAIN base64_ed25519_pubkey TEXT CHECK(VALUE ~ '^[A-Za-z0-9+/]{43}=$');
CREATE DOMAIN oasis_addr TEXT CHECK(length(VALUE) = 46 AND VALUE ~ '^oasis1');

-- Block Data
CREATE TABLE oasis_3.blocks
(
  height     UINT63 PRIMARY KEY,
  block_hash HEX64 NOT NULL,
  time       TIMESTAMP WITH TIME ZONE NOT NULL,

  -- State Root Info
  namespace TEXT NOT NULL,
  version   UINT63 NOT NULL,
  type      TEXT NOT NULL,  -- "invalid" | "state-root" | "io-root"; From https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/storage/mkvs/node/node.go#L68-L68
  root_hash HEX64 NOT NULL,

  beacon     BYTEA,
  metadata   JSON
);

CREATE TABLE oasis_3.transactions
(
  block BIGINT NOT NULL REFERENCES oasis_3.blocks(height) DEFERRABLE INITIALLY DEFERRED,

  txn_hash   HEX64 NOT NULL,
  txn_index  UINT31,
  nonce      UINT63 NOT NULL,
  fee_amount UINT_NUMERIC,
  max_gas    UINT_NUMERIC,
  method     TEXT NOT NULL,
  sender     oasis_addr NOT NULL,
  body       BYTEA,

  -- Error Fields
  -- This includes an encoding of no error.
  module  TEXT,
  code    UINT31,  -- From https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/consensus/api/transaction/results/results.go#L20-L20
  message TEXT,

  -- We require a composite primary key since duplicate transactions can
  -- be included within blocks for this chain.
  PRIMARY KEY (block, txn_hash, txn_index)
);
-- Queries by sender are common, and unusably slow without an index.
CREATE INDEX ix_transactions_sender ON oasis_3.transactions (sender);

CREATE TABLE oasis_3.events
(
  backend TEXT NOT NULL,  -- E.g. registry, staking
  type    TEXT NOT NULL,  -- Enum with many values, see https://github.com/oasisprotocol/oasis-indexer/blob/89b68717205809b491d7926533d096444611bd6b/analyzer/api.go#L171-L171
  body    JSON,

  txn_block  UINT63 NOT NULL,
  txn_hash   HEX64 NOT NULL,
  txn_index  UINT31,

  FOREIGN KEY (txn_block, txn_hash, txn_index) REFERENCES oasis_3.transactions(block, txn_hash, txn_index) DEFERRABLE INITIALLY DEFERRED
);

-- Beacon Backend Data

CREATE TABLE oasis_3.epochs
(
  id           UINT63 PRIMARY KEY,
  start_height UINT63 NOT NULL,
  end_height   UINT63 CHECK (end_height IS NULL OR end_height >= start_height),
  UNIQUE (start_height, end_height)
);

-- Registry Backend Data
CREATE TABLE oasis_3.entities
(
  id      base64_ed25519_pubkey PRIMARY KEY,
  address oasis_addr,
  -- Signed statements about the entity from https://github.com/oasisprotocol/metadata-registry
  meta    JSON
);

CREATE TABLE oasis_3.nodes
(
  -- `id` technically REFERENCES oasis_3.claimed_nodes(node_id) because node had to be pre-claimed; see oasis_3.claimed_nodes.
  -- However, postgres does not allow foreign keys to a non-unique column.
  id         base64_ed25519_pubkey PRIMARY KEY,
  -- Owning entity. The entity has likely claimed this node (see oasis_3.claimed_nodes) previously. However
  -- historically (as per @Yawning), we also allowed node registrations that are signed with the entity signing key,
  -- in which case, the node would be allowed to register without having been pre-claimed by the entity.
  -- For those cases, (id, entity_id) is not a foreign key into oasis_3.claimed_nodes.
  -- Similarly, an entity can un-claim a node after the node registered, but the node can remain be registered for a while.
  entity_id  base64_ed25519_pubkey NOT NULL REFERENCES oasis_3.entities(id),
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
  voting_power     UINT63 DEFAULT 0

  -- TODO: Track node status.
);

-- Claims of entities that they own nodes. Each entity claims 0 or more nodes when it registers.
-- A node can only register if it declares itself to be owned by an entity that previously claimed it.
CREATE TABLE oasis_3.claimed_nodes
(
  entity_id base64_ed25519_pubkey NOT NULL REFERENCES oasis_3.entities(id) DEFERRABLE INITIALLY DEFERRED,
  node_id   base64_ed25519_pubkey NOT NULL,  -- No REFERENCES because the node likely does not exist (in the indexer) yet when the entity claims it.

  PRIMARY KEY (entity_id, node_id)  
);

CREATE TABLE oasis_3.runtimes
(
  id           HEX64 PRIMARY KEY,
  suspended    BOOLEAN NOT NULL DEFAULT false,
  kind         TEXT NOT NULL,  -- "invalid" | "compute" | "manager"; see https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/registry/api/runtime.go#L54-L54
  tee_hardware TEXT NOT NULL,  -- "invalid" | "intel-sgx"; see https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/common/node/node.go#L474-L474
  key_manager  HEX64
);

-- Staking Backend Data

CREATE TABLE oasis_3.accounts
(
  address oasis_addr PRIMARY KEY,

  -- General Account
  general_balance NUMERIC DEFAULT 0,
  nonce           UINT63 NOT NULL DEFAULT 0,

  -- Escrow Account
  escrow_balance_active         UINT_NUMERIC NOT NULL DEFAULT 0,
  escrow_total_shares_active    UINT_NUMERIC NOT NULL DEFAULT 0,
  escrow_balance_debonding      UINT_NUMERIC NOT NULL DEFAULT 0,
  escrow_total_shares_debonding UINT_NUMERIC NOT NULL DEFAULT 0

  -- TODO: Track commission schedule and staking accumulator.
);

CREATE TABLE oasis_3.allowances
(
  owner       oasis_addr NOT NULL REFERENCES oasis_3.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  -- When creating an allowance for the purpose of subsequently depositing funds to a
  -- paratime account A in paratime P (i.e. the expected use case for allowances), `beneficiary` is
  -- the "staking account" of P. The staking account is a special account derived from the paratime ID:
  --  - derivation: https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/staking/api/address.go#L96-L96
  --  - precomputed accounts: https://github.com/oasisprotocol/oasis-wallet-web/blob/34fdf495de5ca0d585addf0073f6a71bba556588/src/config.ts#L89-L139
  beneficiary oasis_addr,
  allowance   UINT_NUMERIC,

  PRIMARY KEY (owner, beneficiary)
);

CREATE TABLE oasis_3.commissions
(
  address  oasis_addr PRIMARY KEY NOT NULL REFERENCES oasis_3.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  schedule JSON
);

CREATE TABLE oasis_3.delegations
(
  delegatee oasis_addr NOT NULL REFERENCES oasis_3.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  delegator oasis_addr NOT NULL REFERENCES oasis_3.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  shares    UINT_NUMERIC NOT NULL,

  PRIMARY KEY (delegatee, delegator)
);

CREATE TABLE oasis_3.debonding_delegations
(
  id         BIGSERIAL PRIMARY KEY,  -- index-internal ID
  delegatee  oasis_addr NOT NULL REFERENCES oasis_3.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  delegator  oasis_addr NOT NULL REFERENCES oasis_3.accounts(address) DEFERRABLE INITIALLY DEFERRED,
  shares     UINT_NUMERIC NOT NULL,
  debond_end UINT63 NOT NULL  -- EpochTime, i.e. number of epochs since base epoch
);

-- Scheduler Backend Data

CREATE TABLE oasis_3.committee_members
(
  node      TEXT NOT NULL,
  valid_for UINT63 NOT NULL,
  runtime   TEXT NOT NULL,
  kind      TEXT NOT NULL,
  role      TEXT NOT NULL,

  PRIMARY KEY (node, runtime, kind, role)
);

-- Governance Backend Data

CREATE TABLE oasis_3.proposals
(
  id            UINT63 PRIMARY KEY,
  submitter     oasis_addr NOT NULL,
  state         TEXT NOT NULL DEFAULT 'active',  -- "active" | "passed" | "rejected" | "failed"; see https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/governance/api/proposal.go#L29-L29
  executed      BOOLEAN NOT NULL DEFAULT false,
  deposit       NUMERIC NOT NULL,

  -- If this proposal is a new proposal.
  handler            TEXT,
  cp_target_version  TEXT,
  rhp_target_version TEXT,
  rcp_target_version TEXT,
  upgrade_epoch      UINT63,

  -- If this proposal cancels an existing proposal.
  cancels BIGINT REFERENCES oasis_3.proposals(id) DEFAULT NULL,

  created_at    UINT63 NOT NULL,  -- EpochTime, i.e. number of epochs since base epoch
  closes_at     UINT63 NOT NULL,  -- EpochTime, i.e. number of epochs since base epoch
  invalid_votes UINT_NUMERIC NOT NULL DEFAULT 0
);

CREATE TABLE oasis_3.votes
(
  proposal UINT63 NOT NULL REFERENCES oasis_3.proposals(id) DEFERRABLE INITIALLY DEFERRED,
  voter    oasis_addr NOT NULL,
  vote     TEXT,  -- "yes" | "no" | "abstain"; see https://github.com/oasisprotocol/oasis-core/blob/f95186e3f15ec64bdd36493cde90be359bd17da8/go/registry/api/runtime.go#L54-L54

  PRIMARY KEY (proposal, voter)
);

-- Indexing Progress Management
CREATE TABLE oasis_3.processed_blocks
(
  height         UINT63 NOT NULL,
  analyzer       TEXT NOT NULL,
  processed_time TIMESTAMP WITH TIME ZONE NOT NULL,

  PRIMARY KEY (height, analyzer)
);

COMMIT;
