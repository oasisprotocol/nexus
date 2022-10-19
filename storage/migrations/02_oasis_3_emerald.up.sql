-- Indexer state initialization for the Emerald ParaTime, after the Damask Upgrade.

BEGIN;

CREATE TABLE oasis_3.emerald_rounds
(
  round       BIGINT PRIMARY KEY,
  version     BIGINT NOT NULL,
  timestamp TIMESTAMP WITH TIME ZONE NOT NULL,

  block_hash      TEXT NOT NULL,
  prev_block_hash TEXT NOT NULL,

  io_root          TEXT NOT NULL,
  state_root       TEXT NOT NULL,
  messages_hash    TEXT NOT NULL,
  in_messages_hash TEXT NOT NULL,

  num_transactions INTEGER NOT NULL,
  gas_used         BIGINT NOT NULL,
  size             INTEGER NOT NULL
);

CREATE INDEX ix_emerald_rounds_block_hash ON oasis_3.emerald_rounds USING hash (block_hash);

CREATE TABLE oasis_3.emerald_transactions
(
  round       BIGINT NOT NULL,
  tx_index    INTEGER NOT NULL,
  tx_hash     TEXT NOT NULL,
  tx_eth_hash TEXT,
  -- raw is cbor(UnverifiedTransaction). If you're unable to get a copy of the
  -- transaction from the node itself, parse from here. Remove this if we
  -- later store sufficiently detailed data in other columns or if we turn out
  -- to be able to get a copy of the transaction elsewhere.
  raw         BYTEA NOT NULL,
  PRIMARY KEY (round, tx_index)
);

CREATE INDEX ix_emerald_transactions_tx_hash ON oasis_3.emerald_transactions USING hash (tx_hash);
CREATE INDEX ix_emerald_transactions_tx_eth_hash ON oasis_3.emerald_transactions USING hash (tx_eth_hash);

CREATE TABLE oasis_3.emerald_transaction_signers
(
  round          BIGINT NOT NULL,
  tx_index       INTEGER NOT NULL,
  -- Emerald processes mainly Ethereum-format transactions with only one
  -- signer, but Emerald is built on the Oasis runtime SDK, which supports
  -- multiple signers on a transaction (note that this is a distinct concept
  -- from multisig accounts).
  signer_index   INTEGER NOT NULL,
  signer_address TEXT NOT NULL,
  nonce          BIGINT NOT NULL,
  PRIMARY KEY (round, tx_index, signer_index)
);

CREATE INDEX ix_emerald_transaction_signers_signer_address_signer_nonce ON oasis_3.emerald_transaction_signers (signer_address, nonce);

CREATE TABLE oasis_3.emerald_related_transactions
(
  account_address TEXT NOT NULL,
  tx_round        BIGINT NOT NULL,
  tx_index        INTEGER NOT NULL
);

CREATE INDEX ix_emerald_related_transactions_address_height_index ON oasis_3.emerald_related_transactions (account_address, tx_round, tx_index);

-- Oasis addresses are derived from a derivation "context" and a piece of
-- data, such as an ed25519 public key or an Ethereum address. The derivation
-- is one-way, so you'd have to look up the address in this table and see if
-- we've encountered the preimage before to find out what the address was
-- derived from.
--
-- If you need to go the other way, from context + data to address, you'd just
-- run the derivation. Thus we don't provide an index for going that way. See
-- oasis-core/go/common/crypto/address/address.go for details. Consider
-- inserting the preimage here if you're ingesting new blockchain data though.
--
-- Retain this across hard forks as long as the address derivation scheme is
-- compatible.
CREATE TABLE oasis_3.address_preimages
(
    -- address is the Bech32-encoded Oasis address (i.e. starting with
    -- oasis1...).
    address            TEXT NOT NULL PRIMARY KEY,
    context_identifier TEXT NOT NULL,
    context_version    INTEGER NOT NULL,
    address_data       BYTEA NOT NULL
);

-- Core Module Data
CREATE TABLE oasis_3.emerald_gas_used
(
  round  BIGINT NOT NULL,
  sender TEXT NOT NULL,
  amount NUMERIC NOT NULL
);

CREATE INDEX ix_emerald_gas_used_sender ON oasis_3.emerald_gas_used(sender);

-- Accounts Module Data

-- The emerald_transfers table encapsulates transfers, burns, and mints.
-- Burns are denoted by the 0-address as the receiver and mints are
-- denoted by the 0-address as the sender.
CREATE TABLE oasis_3.emerald_transfers
(
  round    BIGINT NOT NULL,
  sender   TEXT NOT NULL DEFAULT '0',
  receiver TEXT NOT NULL DEFAULT '0',
  amount   TEXT NOT NULL
);

CREATE INDEX ix_emerald_transfers_sender ON oasis_3.emerald_transfers(sender);
CREATE INDEX ix_emerald_transfers_receiver ON oasis_3.emerald_transfers(receiver);

-- Consensus Accounts Module Data
CREATE TABLE oasis_3.emerald_deposits
(
  round    BIGINT NOT NULL,
  sender   TEXT NOT NULL,
  receiver TEXT NOT NULL,
  amount   TEXT NOT NULL,
  nonce    BIGINT NOT NULL,

  -- Optional error data
  module TEXT,
  code   BIGINT
);

CREATE INDEX ix_emerald_deposits_sender ON oasis_3.emerald_deposits(sender);
CREATE INDEX ix_emerald_deposits_receiver ON oasis_3.emerald_deposits(receiver);

CREATE TABLE oasis_3.emerald_withdraws
(
  round    BIGINT NOT NULL,
  sender   TEXT NOT NULL,
  receiver TEXT NOT NULL,
  amount   TEXT NOT NULL,
  nonce    BIGINT NOT NULL,

  -- Optional error data
  module TEXT,
  code   BIGINT
);

CREATE INDEX ix_emerald_withdraws_sender ON oasis_3.emerald_withdraws(sender);
CREATE INDEX ix_emerald_withdraws_receiver ON oasis_3.emerald_withdraws(receiver);

COMMIT;
