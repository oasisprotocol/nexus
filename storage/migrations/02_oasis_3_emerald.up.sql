-- Indexer state initialization for the Emerald ParaTime, after the Damask Upgrade.

BEGIN;

CREATE TABLE oasis_3.emerald_rounds
(
  round     UINT63 PRIMARY KEY,
  version   UINT63 NOT NULL,
  timestamp TIMESTAMP WITH TIME ZONE NOT NULL,

  block_hash      HEX64 NOT NULL, -- Hash of this round's block. Does not reference consensus.
  prev_block_hash HEX64 NOT NULL,

  io_root          HEX64 NOT NULL,
  state_root       HEX64 NOT NULL,
  messages_hash    HEX64 NOT NULL,
  in_messages_hash HEX64 NOT NULL,

  num_transactions UINT31 NOT NULL,
  gas_used         UINT63 NOT NULL,
  size             UINT31 NOT NULL  -- Total byte size of all transactions in the block.
);

CREATE INDEX ix_emerald_rounds_block_hash ON oasis_3.emerald_rounds USING hash (block_hash);
CREATE INDEX ix_emerald_rounds_timestamp ON oasis_3.emerald_rounds (timestamp);

CREATE TABLE oasis_3.emerald_transactions
(
  round       UINT63 NOT NULL REFERENCES oasis_3.emerald_rounds DEFERRABLE INITIALLY DEFERRED,
  tx_index    UINT31 NOT NULL,
  tx_hash     HEX64 NOT NULL,
  tx_eth_hash HEX64,
  -- raw is cbor(UnverifiedTransaction). If you're unable to get a copy of the
  -- transaction from the node itself, parse from here. Remove this if we
  -- later store sufficiently detailed data in other columns or if we turn out
  -- to be able to get a copy of the transaction elsewhere.
  raw         BYTEA NOT NULL,
  -- result_raw is cbor(CallResult).
  result_raw  BYTEA NOT NULL,
  PRIMARY KEY (round, tx_index)
);

CREATE INDEX ix_emerald_transactions_tx_hash ON oasis_3.emerald_transactions USING hash (tx_hash);
CREATE INDEX ix_emerald_transactions_tx_eth_hash ON oasis_3.emerald_transactions USING hash (tx_eth_hash);

CREATE TABLE oasis_3.emerald_transaction_signers
(
  round          UINT63 NOT NULL,
  tx_index       UINT31 NOT NULL,
  -- Emerald processes mainly Ethereum-format transactions with only one
  -- signer, but Emerald is built on the Oasis runtime SDK, which supports
  -- multiple signers on a transaction (note that this is a distinct concept
  -- from multisig accounts).
  signer_index   UINT31 NOT NULL,
  signer_address oasis_addr NOT NULL,
  nonce          UINT31 NOT NULL,
  PRIMARY KEY (round, tx_index, signer_index),
  FOREIGN KEY (round, tx_index) REFERENCES oasis_3.emerald_transactions(round, tx_index) DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX ix_emerald_transaction_signers_signer_address_signer_nonce ON oasis_3.emerald_transaction_signers (signer_address, nonce);

CREATE TABLE oasis_3.emerald_related_transactions
(
  account_address oasis_addr NOT NULL,
  tx_round        UINT63 NOT NULL,
  tx_index        UINT31 NOT NULL,
  FOREIGN KEY (tx_round, tx_index) REFERENCES oasis_3.emerald_transactions(round, tx_index) DEFERRABLE INITIALLY DEFERRED
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
--
-- The data is accumulated from signed paratime txs; signer info contains all the
-- fields in this table (so that the signature can be verified correctly).
CREATE TABLE oasis_3.address_preimages
(
  address            oasis_addr NOT NULL PRIMARY KEY,
  -- "oasis-core/address: staking" | "oasis-runtime-sdk/address: secp256k1eth" | "oasis-runtime-sdk/address: sr25519" | "oasis-runtime-sdk/address: multisig" | "oasis-runtime-sdk/address: module"
  -- From https://github.com/oasisprotocol/oasis-sdk/blob/386ba0b99fcd1425c68015e0033a462d9a577835/client-sdk/go/types/address.go#L22-L22
  context_identifier TEXT NOT NULL,
  context_version    UINT31 NOT NULL,
  -- Data from which the address was derived. For example, for a "secp256k1eth" context, this is the
  -- Ethereum address. For a "staking" context, this is the ed25519 pubkey.
  address_data       BYTEA NOT NULL
);

CREATE TABLE oasis_3.emerald_token_balances
(
  token_address TEXT NOT NULL,
  account_address TEXT NOT NULL,
  balance NUMERIC(1000,0) NOT NULL,
  PRIMARY KEY (token_address, account_address)
);

CREATE INDEX ix_emerald_token_address ON oasis_3.emerald_token_balances (token_address) WHERE balance != 0;

-- Core Module Data
CREATE TABLE oasis_3.emerald_gas_used
(
  round  UINT63 NOT NULL REFERENCES oasis_3.emerald_rounds DEFERRABLE INITIALLY DEFERRED,
  sender oasis_addr REFERENCES oasis_3.address_preimages,  -- TODO: add NOT NULL; but analyzer is only putting NULLs here for now because it doesn't have the data
  amount NUMERIC NOT NULL
);

CREATE INDEX ix_emerald_gas_used_sender ON oasis_3.emerald_gas_used(sender);

-- Accounts Module Data

-- The emerald_transfers table encapsulates transfers, burns, and mints.
-- Burns are denoted by NULL as the receiver and mints are denoted by NULL as the sender.
CREATE TABLE oasis_3.emerald_transfers
(
  round    UINT63 NOT NULL REFERENCES oasis_3.emerald_rounds DEFERRABLE INITIALLY DEFERRED,
  -- Any paratime account. This almost always REFERENCES oasis_3.address_preimages(address)
  -- because the sender signed the Transfer tx and was inserted into address_preimages then.
  -- Exceptions are special addresses; see e.g. the rewards-pool address.
  -- TODO: oasis1qr677rv0dcnh7ys4yanlynysvnjtk9gnsyhvm6ln is known to violate FK; where does it come from? Register it in prework().
  -- If NULL, this is a mint.
  sender   oasis_addr,
  -- Any paratime account. Not necessarily found in oasis_3.address_preimages, e.g. if
  -- this account hasn't signed a tx yet.
  -- If NULL, this is a burn.
  receiver oasis_addr,
  amount   UINT_NUMERIC NOT NULL,

  CHECK (NOT (sender IS NULL AND receiver IS NULL))
);

CREATE INDEX ix_emerald_transfers_sender ON oasis_3.emerald_transfers(sender);
CREATE INDEX ix_emerald_transfers_receiver ON oasis_3.emerald_transfers(receiver);

-- Consensus Accounts Module Data
-- Deposits from the consensus layer into the paratime.
CREATE TABLE oasis_3.emerald_deposits
(
  round    UINT63 NOT NULL REFERENCES oasis_3.emerald_rounds DEFERRABLE INITIALLY DEFERRED,
  -- The `sender` is a consensus account, so this REFERENCES oasis_3.accounts; we omit the FK so
  -- that consensus and paratimes can be indexed independently.
  -- It also REFERENCES oasis_3.address_preimages because the sender signed at least the Deposit tx.
  sender   oasis_addr NOT NULL REFERENCES oasis_3.address_preimages DEFERRABLE INITIALLY DEFERRED,
  -- The `receiver` can be any oasis1-form paratime address.
  -- With EVM paratimes, two types of deposits are common:
  --  * Deposit intends to credit a hex-form (eth) account. The user first derives the oasis1-form address,
  --    then uses it as `receiver`. Such a `receiver` address has a secp256k1eth key, an is not valid in
  --    consensus (which only uses ed25519), meaning there is no FK in oasis_3.addresses.
  --  * Deposit intends to credit an ed25519-backed account. These accounts do not exist in classic Ethereum.
  --    The associated hex-form Ethereum address either does not exist or is not knowable.
  -- Regardless, the `receiver` often REFERENCES oasis_3.address_preimages(address), a notable exception
  -- being if the target account hasn't signed any txs yet.
  receiver oasis_addr NOT NULL,
  amount   UINT_NUMERIC NOT NULL,
  nonce    UINT63 NOT NULL,

  -- Optional error data; from https://github.com/oasisprotocol/oasis-sdk/blob/386ba0b99fcd1425c68015e0033a462d9a577835/client-sdk/go/modules/consensusaccounts/types.go#L44-L44
  module TEXT,
  code   UINT63
);

CREATE INDEX ix_emerald_deposits_sender ON oasis_3.emerald_deposits(sender);
CREATE INDEX ix_emerald_deposits_receiver ON oasis_3.emerald_deposits(receiver);

-- Withdrawals from the paratime into consensus layer.
CREATE TABLE oasis_3.emerald_withdraws
(
  round    UINT63 NOT NULL REFERENCES oasis_3.emerald_rounds DEFERRABLE INITIALLY DEFERRED,
  -- The `sender` can be any paratime address. (i.e. secp256k1eth-backed OR ed25519-backed;
  -- other are options unlikely in an EVM paratime)
  -- It REFERENCES oasis_3.address_preimages because the sender signed at least the Withdraw tx.
  sender   oasis_addr NOT NULL REFERENCES oasis_3.address_preimages DEFERRABLE INITIALLY DEFERRED,
  -- The `receiver` is a consensus account, so this REFERENCES oasis_3.accounts; we omit the FK so
  -- that consensus and paratimes can be indexed independently.
  receiver oasis_addr NOT NULL,
  amount   UINT_NUMERIC NOT NULL,
  nonce    UINT63 NOT NULL,

  -- Optional error data
  module TEXT,
  code   UINT63
);

CREATE INDEX ix_emerald_withdraws_sender ON oasis_3.emerald_withdraws(sender);
CREATE INDEX ix_emerald_withdraws_receiver ON oasis_3.emerald_withdraws(receiver);

-- Grant others read-only use.
-- (We granted already in 01_oasis_3_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA oasis_3 TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA oasis_3 TO PUBLIC;

COMMIT;
