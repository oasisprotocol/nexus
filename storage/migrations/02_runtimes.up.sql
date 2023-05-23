-- Indexer state initialization for the Emerald ParaTime, after the Damask Upgrade.

BEGIN;

CREATE TYPE runtime AS ENUM ('emerald', 'sapphire', 'cipher');
CREATE TYPE call_format AS ENUM ('encrypted/x25519-deoxysii');

CREATE TABLE chain.runtime_blocks
(
  runtime   runtime NOT NULL,
  round     UINT63,
  PRIMARY KEY (runtime, round),
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
CREATE INDEX ix_runtime_blocks_block_hash ON chain.runtime_blocks USING hash (block_hash);  -- Hash indexes cannot span two colmns (runtime, block_hash). Not a problem for efficiency because block_hash is globally uniqueish.
CREATE INDEX ix_runtime_blocks_timestamp ON chain.runtime_blocks (runtime, timestamp);
CREATE INDEX ix_runtime_blocks_round ON chain.runtime_blocks (runtime, round);

CREATE TABLE chain.runtime_transactions
(
  runtime     runtime NOT NULL,
  round       UINT63 NOT NULL,
  FOREIGN KEY (runtime, round) REFERENCES chain.runtime_blocks DEFERRABLE INITIALLY DEFERRED,
  tx_index    UINT31 NOT NULL,
  PRIMARY KEY (runtime, round, tx_index),
  timestamp TIMESTAMP WITH TIME ZONE NOT NULL,

  tx_hash     HEX64 NOT NULL,
  tx_eth_hash HEX64,
  -- NOTE: The signer(s) and their nonce(s) are stored separately in runtime_transaction_signers.

  fee         UINT_NUMERIC NOT NULL,
  gas_limit   UINT63 NOT NULL,
  gas_used    UINT63 NOT NULL,

  size UINT31 NOT NULL,

  -- Transaction contents.
  method      TEXT,         -- accounts.Transfer, consensus.Deposit, consensus.Withdraw, evm.Create, evm.Call. NULL for malformed and encrypted txs.
  body        JSONB,        -- For EVM txs, the EVM method and args are encoded in here. NULL for malformed and encrypted txs.
  "to"        oasis_addr,   -- Exact semantics depend on method. Extracted from body; for convenience only.
  amount      UINT_NUMERIC, -- Exact semantics depend on method. Extracted from body; for convenience only.

  -- Encrypted data in encrypted Ethereum-format transactions.
  evm_encrypted_format call_format,
  evm_encrypted_public_key BYTEA,
  evm_encrypted_data_nonce BYTEA,
  evm_encrypted_data_data BYTEA,
  evm_encrypted_result_nonce BYTEA,
  evm_encrypted_result_data BYTEA,

  -- Error information.
  success       BOOLEAN,  -- NULL means success is unknown (can happen in confidential runtimes)
  error_module  TEXT,
  error_code    UINT63,
  error_message TEXT
);
CREATE INDEX ix_runtime_transactions_tx_hash ON chain.runtime_transactions USING hash (tx_hash);
CREATE INDEX ix_runtime_transactions_tx_eth_hash ON chain.runtime_transactions USING hash (tx_eth_hash);
CREATE INDEX ix_runtime_transactions_timestamp ON chain.runtime_transactions (runtime, timestamp);

CREATE TABLE chain.runtime_transaction_signers
(
  runtime        runtime NOT NULL,
  round          UINT63 NOT NULL,
  tx_index       UINT31 NOT NULL,
  -- Emerald processes mainly Ethereum-format transactions with only one
  -- signer, but Emerald is built on the Oasis runtime SDK, which supports
  -- multiple signers on a transaction (note that this is a distinct concept
  -- from multisig accounts).
  signer_index   UINT31 NOT NULL,
  signer_address oasis_addr NOT NULL,
  nonce          UINT63 NOT NULL,
  PRIMARY KEY (runtime, round, tx_index, signer_index),
  FOREIGN KEY (runtime, round, tx_index) REFERENCES chain.runtime_transactions(runtime, round, tx_index) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX ix_runtime_transaction_signers_address_nonce ON chain.runtime_transaction_signers (runtime, signer_address, nonce);

CREATE TABLE chain.runtime_related_transactions
(
  runtime         runtime NOT NULL,
  account_address oasis_addr NOT NULL,
  tx_round        UINT63 NOT NULL,
  tx_index        UINT31 NOT NULL,
  FOREIGN KEY (runtime, tx_round, tx_index) REFERENCES chain.runtime_transactions(runtime, round, tx_index) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX ix_runtime_related_transactions_address ON chain.runtime_related_transactions (runtime, account_address);
CREATE INDEX ix_runtime_related_transactions_round_index ON chain.runtime_related_transactions (runtime, tx_round, tx_index);
CREATE INDEX ix_runtime_related_transactions_address_round_index ON chain.runtime_related_transactions (runtime, account_address, tx_round, tx_index);

-- Events emitted from the runtimes. Includes deeply-parsed EVM events from EVM runtimes.
CREATE TABLE chain.runtime_events
(
  runtime runtime NOT NULL,
  round UINT63 NOT NULL,
  tx_index UINT31,
  FOREIGN KEY (runtime, round, tx_index) REFERENCES chain.runtime_transactions(runtime, round, tx_index) DEFERRABLE INITIALLY DEFERRED,

  tx_hash HEX64,
  -- TODO: add link to openapi spec section with runtime event types.
  type TEXT NOT NULL,
  -- The raw event, as returned by the oasis-sdk runtime client.
  -- `evm.log` events are further parsed into known event types,
  -- e.g. (ERC20) Transfer, to populate the `evm_log_name` and
  -- `evm_log_params` fields below.
  body JSONB NOT NULL,
  evm_log_name TEXT,
  -- The event signature, if it exists, will be the first topic.
  evm_log_signature TEXT GENERATED ALWAYS AS (body->'topics'->>0) STORED,
  evm_log_params JSONB,
  related_accounts TEXT[]
);
CREATE INDEX ix_runtime_events_round ON chain.runtime_events(runtime, round);  -- for sorting by round, when there are no filters applied
CREATE INDEX ix_runtime_events_tx_hash ON chain.runtime_events(tx_hash);
CREATE INDEX ix_runtime_events_related_accounts ON chain.runtime_events USING gin(related_accounts);
CREATE INDEX ix_runtime_events_evm_log_signature ON chain.runtime_events(evm_log_signature);
CREATE INDEX ix_runtime_events_evm_log_params ON chain.runtime_events USING gin(evm_log_params);

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
CREATE TABLE chain.address_preimages
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

-- -- -- -- -- -- -- -- -- -- -- -- -- Module evm -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --

CREATE TABLE chain.evm_token_balances
(
  runtime runtime NOT NULL,
  token_address oasis_addr NOT NULL,
  account_address oasis_addr NOT NULL,
  PRIMARY KEY (runtime, token_address, account_address),
  -- Allow signed values because contracts may overdraw accounts beyond our
  -- understanding or may misbehave.
  balance NUMERIC(1000,0) NOT NULL
);
CREATE INDEX ix_evm_token_address ON chain.evm_token_balances (runtime, token_address) WHERE balance != 0;

CREATE TABLE chain.evm_tokens
(
  runtime runtime NOT NULL,
  token_address oasis_addr NOT NULL,
  token_type INTEGER NOT NULL, -- 0 = unsupported, X = ERC-X; full spec at https://github.com/oasisprotocol/oasis-indexer/blob/v0.0.16/analyzer/runtime/evm.go#L21
  token_name TEXT,
  symbol TEXT,
  decimals INTEGER,
  total_supply uint_numeric,
  PRIMARY KEY (runtime, token_address)
);

CREATE TABLE chain.evm_token_analysis
(
  runtime runtime NOT NULL,
  token_address oasis_addr NOT NULL,
  -- Block analyzer bumps this when it sees the mutable fields of the token
  -- change (e.g. total supply) based on dead reckoning.
  last_mutate_round UINT63 NOT NULL,
  -- Token analyzer bumps this when it downloads info about the token.
  last_download_round UINT63,
  PRIMARY KEY (runtime, token_address)
);
CREATE INDEX ix_evm_token_analysis_stale ON chain.evm_token_analysis (runtime, token_address) WHERE last_download_round IS NULL OR last_mutate_round > last_download_round;

CREATE TABLE chain.evm_token_balance_analysis
(
  runtime runtime NOT NULL,
  token_address oasis_addr NOT NULL,
  account_address oasis_addr NOT NULL,
  last_mutate_round UINT63 NOT NULL,
  last_download_round UINT63,
  PRIMARY KEY (runtime, token_address, account_address)
);
CREATE INDEX ix_evm_token_balance_analysis_stale ON chain.evm_token_balance_analysis (runtime, token_address, account_address) WHERE last_download_round IS NULL OR last_mutate_round > last_download_round;

-- -- -- -- -- -- -- -- -- -- -- -- -- Module accounts -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --

-- This table encapsulates transfers, burns, and mints (at the level of the `accounts` SDK module; NOT evm transfers).
-- Burns are denoted by NULL as the receiver and mints are denoted by NULL as the sender.
CREATE TABLE chain.runtime_transfers
(
  runtime  runtime NOT NULL,
  round    UINT63 NOT NULL,
  FOREIGN KEY (runtime, round) REFERENCES chain.runtime_blocks DEFERRABLE INITIALLY DEFERRED,
  -- Any paratime account. This almost always REFERENCES chain.address_preimages(address)
  -- because the sender signed the Transfer tx and was inserted into address_preimages then.
  -- Exceptions are special addresses; see e.g. the rewards-pool address.
  -- TODO: oasis1qr677rv0dcnh7ys4yanlynysvnjtk9gnsyhvm6ln is known to violate FK; where does it come from? Register it in prework().
  -- If NULL, this is a mint.
  sender   oasis_addr,
  -- Any paratime account. Not necessarily found in chain.address_preimages, e.g. if
  -- this account hasn't signed a tx yet.
  -- If NULL, this is a burn.
  receiver oasis_addr,
  symbol   TEXT NOT NULL,  -- called `Denomination` in the SDK
  amount   UINT_NUMERIC NOT NULL,

  CHECK (NOT (sender IS NULL AND receiver IS NULL))
);
CREATE INDEX ix_runtime_transfers_sender ON chain.runtime_transfers(sender);
CREATE INDEX ix_runtime_transfers_receiver ON chain.runtime_transfers(receiver);

-- -- -- -- -- -- -- -- -- -- -- -- -- Module consensusaccounts -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --

-- Deposits from the consensus layer into the paratime.
CREATE TABLE chain.runtime_deposits
(
  runtime  runtime NOT NULL,
  round    UINT63 NOT NULL,
  FOREIGN KEY (runtime, round) REFERENCES chain.runtime_blocks DEFERRABLE INITIALLY DEFERRED,
  -- The `sender` is a consensus account, so this REFERENCES chain.accounts; we omit the FK so
  -- that consensus and paratimes can be indexed independently.
  -- It also REFERENCES chain.address_preimages because the sender signed at least the Deposit tx.
  sender   oasis_addr NOT NULL REFERENCES chain.address_preimages DEFERRABLE INITIALLY DEFERRED,
  -- The `receiver` can be any oasis1-form paratime address.
  -- With EVM paratimes, two types of deposits are common:
  --  * Deposit intends to credit a hex-form (eth) account. The user first derives the oasis1-form address,
  --    then uses it as `receiver`. Such a `receiver` address has a secp256k1eth key, an is not valid in
  --    consensus (which only uses ed25519), meaning there is no FK in chain.addresses.
  --  * Deposit intends to credit an ed25519-backed account. These accounts do not exist in classic Ethereum.
  --    The associated hex-form Ethereum address either does not exist or is not knowable.
  -- Regardless, the `receiver` often REFERENCES chain.address_preimages(address), a notable exception
  -- being if the target account hasn't signed any txs yet.
  receiver oasis_addr NOT NULL,
  amount   UINT_NUMERIC NOT NULL,  -- always in native denomination
  nonce    UINT63 NOT NULL,

  -- Optional error data; from https://github.com/oasisprotocol/oasis-sdk/blob/386ba0b99fcd1425c68015e0033a462d9a577835/client-sdk/go/modules/consensusaccounts/types.go#L44-L44
  module TEXT,
  code   UINT63
);
CREATE INDEX ix_runtime_deposits_sender ON chain.runtime_deposits(sender);
CREATE INDEX ix_runtime_deposits_receiver ON chain.runtime_deposits(receiver);

-- Withdrawals from the paratime into consensus layer.
CREATE TABLE chain.runtime_withdraws
(
  runtime  runtime NOT NULL,
  round    UINT63 NOT NULL,
  FOREIGN KEY (runtime, round) REFERENCES chain.runtime_blocks DEFERRABLE INITIALLY DEFERRED,
  -- The `sender` can be any paratime address. (i.e. secp256k1eth-backed OR ed25519-backed;
  -- other are options unlikely in an EVM paratime)
  -- It REFERENCES chain.address_preimages because the sender signed at least the Withdraw tx.
  sender   oasis_addr NOT NULL REFERENCES chain.address_preimages DEFERRABLE INITIALLY DEFERRED,
  -- The `receiver` is a consensus account, so this REFERENCES chain.accounts; we omit the FK so
  -- that consensus and paratimes can be indexed independently.
  receiver oasis_addr NOT NULL,
  amount   UINT_NUMERIC NOT NULL,  -- always in native denomination
  nonce    UINT63 NOT NULL,

  -- Optional error data
  module TEXT,
  code   UINT63
);
CREATE INDEX ix_runtime_withdraws_sender ON chain.runtime_withdraws(sender);
CREATE INDEX ix_runtime_withdraws_receiver ON chain.runtime_withdraws(receiver);

-- Balance of the oasis-sdk native tokens (notably ROSE) in paratimes.
CREATE TABLE chain.runtime_sdk_balances (
  runtime runtime,
  account_address oasis_addr,
  symbol   TEXT NOT NULL,  -- called `Denomination` in the SDK
  PRIMARY KEY (runtime, account_address, symbol),
  balance NUMERIC(1000,0) NOT NULL  -- TODO: Use UINT_NUMERIC once we are processing Emerald from round 0.
);

-- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --

-- Grant others read-only use.
-- (We granted already in 01_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
