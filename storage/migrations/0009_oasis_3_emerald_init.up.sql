-- Indexer state initialization for the Emerald ParaTime, after the Damask Upgrade.

BEGIN;

CREATE TABLE IF NOT EXISTS oasis_3.emerald_rounds
(
  height    NUMERIC PRIMARY KEY,
  version   BIGINT,
  timestamp NUMERIC NOT NULL,

  block_hash      TEXT NOT NULL,
  prev_block_hash TEXT NOT NULL,

  io_root          TEXT NOT NULL,
  state_root       TEXT NOT NULL,
  messages_hash    TEXT NOT NULL,
  in_messages_hash TEXT NOT NULL,

  -- Arbitrary additional data.
  extra_data JSON
);

CREATE TABLE IF NOT EXISTS oasis_3.emerald_transactions
(
  height NUMERIC PRIMARY KEY,

  -- Arbitrary additional data.
  extra_data JSON
);

-- Core Module Data
CREATE TABLE IF NOT EXISTS oasis_3.emerald_gas_used
(
  height NUMERIC NOT NULL,
  sender TEXT NOT NULL,
  amount NUMERIC NOT NULL
);

CREATE INDEX ix_emerald_gas_used_sender ON oasis_3.emerald_gas_used(sender);

-- Accounts Module Data

-- The emerald_transfers table encapsulates transfers, burns, and mints.
-- Burns are denoted by the 0-address as the receiver and mints are
-- denoted by the 0-address as the sender.
CREATE TABLE IF NOT EXISTS oasis_3.emerald_transfers
(
  height   NUMERIC NOT NULL,
  sender   TEXT NOT NULL DEFAULT '0',
  receiver TEXT NOT NULL DEFAULT '0',
  amount   TEXT NOT NULL
);

CREATE INDEX ix_emerald_transfers_sender ON oasis_3.emerald_transfers(sender);
CREATE INDEX ix_emerald_transfers_receiver ON oasis_3.emerald_transfers(receiver);

-- Consensus Accounts Module Data
CREATE TABLE IF NOT EXISTS oasis_3.emerald_deposits
(
  height   NUMERIC NOT NULL,
  sender   TEXT NOT NULL,
  receiver TEXT NOT NULL,
  amount   TEXT NOT NULL,
  nonce    NUMERIC NOT NULL,

  -- Optional error data
  module TEXT,
  code   NUMERIC
);

CREATE INDEX ix_emerald_deposits_sender ON oasis_3.emerald_deposits(sender);
CREATE INDEX ix_emerald_deposits_receiver ON oasis_3.emerald_deposits(receiver);

CREATE TABLE IF NOT EXISTS oasis_3.emerald_withdraws
(
  height   NUMERIC NOT NULL,
  sender   TEXT NOT NULL,
  receiver TEXT NOT NULL,
  amount   TEXT NOT NULL,
  nonce    NUMERIC NOT NULL,

  -- Optional error data
  module TEXT,
  code   NUMERIC
);

CREATE INDEX ix_emerald_withdraws_sender ON oasis_3.emerald_withdraws(sender);
CREATE INDEX ix_emerald_withdraws_receiver ON oasis_3.emerald_withdraws(receiver);

COMMIT;
