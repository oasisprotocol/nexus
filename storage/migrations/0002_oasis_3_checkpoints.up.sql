BEGIN;

-- Staking Backend Checkpoints

CREATE TABLE IF NOT EXISTS oasis_3.accounts_checkpoint
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

CREATE TABLE IF NOT EXISTS oasis_3.allowances_checkpoint
(
  owner       TEXT NOT NULL REFERENCES oasis_3.accounts(address),
  beneficiary TEXT NOT NULL,
  allowance   NUMERIC,

  PRIMARY KEY (owner, beneficiary)
);

CREATE TABLE IF NOT EXISTS oasis_3.delegations_checkpoint
(
  delegatee TEXT NOT NULL,
  delegator TEXT NOT NULL REFERENCES oasis_3.accounts(address),
  shares    NUMERIC NOT NULL,

  PRIMARY KEY (delegatee, delegator)
);

CREATE TABLE IF NOT EXISTS oasis_3.debonding_delegations_checkpoint
(
  delegatee  TEXT NOT NULL,
  delegator  TEXT NOT NULL REFERENCES oasis_3.accounts(address),
  shares     NUMERIC NOT NULL,
  debond_end BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS oasis_3.checkpointed_heights
(
  height          BIGINT PRIMARY KEY,
  checkpoint_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Governance Backend Data

CREATE TABLE IF NOT EXISTS oasis_3.proposals_checkpoint
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
  cancels BIGINT REFERENCES oasis_3.proposals(id) DEFAULT NULL,

  created_at    BIGINT NOT NULL,
  closes_at     BIGINT NOT NULL,
  invalid_votes NUMERIC NOT NULL DEFAULT 0,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS oasis_3.votes_checkpoint
(
  proposal BIGINT NOT NULL REFERENCES oasis_3.proposals(id),
  voter    TEXT NOT NULL,
  vote     TEXT,

  PRIMARY KEY (proposal, voter),

  -- Arbitrary additional data.
  extra_data JSON
);

COMMIT;
