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

  -- TODO: These should also be NOT NULL, but they're populated separately.
  num_transactions INTEGER,
  gas_used         BIGINT,
  size             INTEGER
);

CREATE TABLE IF NOT EXISTS oasis_3.emerald_transactions
(
  round       BIGINT NOT NULL,
  tx_index    INTEGER NOT NULL,
  tx_hash     TEXT NOT NULL,
  tx_eth_hash TEXT,
  PRIMARY KEY (round, tx_index)
);

CREATE INDEX ix_emerald_transactions_tx_hash ON oasis_3.emerald_transactions(tx_hash);
CREATE INDEX ix_emerald_transactions_tx_eth_hash ON oasis_3.emerald_transactions(tx_eth_hash);

CREATE TABLE IF NOT EXISTS oasis_3.emerald_transaction_signers
(
  round          BIGINT NOT NULL,
  tx_index       INTEGER NOT NULL,
  signer_index   INTEGER NOT NULL,
  signer_address TEXT NOT NULL,
  nonce          BIGINT NOT NULL,
  PRIMARY KEY (round, tx_index, signer_index)
);

CREATE INDEX ix_emerald_transaction_signers_signer_address_signer_nonce ON oasis_3.emerald_transaction_signers(signer_address, nonce);

CREATE TABLE IF NOT EXISTS oasis_3.emerald_related_transactions
(
  account_address TEXT NOT NULL,
  tx_round        BIGINT NOT NULL,
  tx_index        INTEGER NOT NULL
);

CREATE INDEX ix_emerald_related_transactions_address_height_index ON oasis_3.emerald_related_transactions(account_address, tx_round, tx_index);

-- Retain this across hard forks as long as the address derivation scheme is compatible.
CREATE TABLE IF NOT EXISTS oasis_3.address_preimages
(
    address            TEXT NOT NULL PRIMARY KEY,
    context_identifier TEXT NOT NULL,
    context_version    INTEGER NOT NULL,
    address_data       BYTEA NOT NULL
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
