-- State initialization for the Emerald ParaTime, after the Damask Upgrade.

BEGIN;

CREATE TYPE public.runtime AS ENUM ('emerald', 'sapphire', 'cipher');
CREATE TYPE public.call_format AS ENUM ('encrypted/x25519-deoxysii');

CREATE TABLE chain.runtime_blocks
(
  runtime   runtime NOT NULL,
  round     UINT63,
  PRIMARY KEY (runtime, round),
  version   UINT63 NOT NULL,
  timestamp TIMESTAMP WITH TIME ZONE NOT NULL,

  block_hash       HEX64 NOT NULL, -- Hash of this round's block. Does not reference consensus.
  prev_block_hash  HEX64 NOT NULL,

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

  -- For evm.Call transactions, we store both the name of the function and
  -- the function parameters. The parameters are stored in an ordered JSON array,
  -- and each each parameter is a JSON object: {name: ..., evm_type: ..., value: ...}
  evm_fn_name TEXT,
  evm_fn_params JSONB CHECK (jsonb_typeof(evm_fn_params)='array'),

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
  error_message TEXT,
  -- The unparsed transaction error message. The "parsed" version will be 
  -- identical in the majority of cases. One notable exception are txs that
  -- were reverted inside the EVM; for those, the raw msg is abi-encoded.
  error_message_raw TEXT,
  -- Custom errors may be arbitrarily defined by the contract abi. This field
  -- stores the full abi-decoded error object. Note that the error name is 
  -- stored separately in the existing error_message column. For example, if we
  -- have an error like `InsufficientBalance{available: 4, required: 10}`.
  -- the error_message column would hold `InsufficientBalance`, and 
  -- the error_params column would store `{available: 4, required: 10}`.
  error_params JSONB,
  -- Internal tracking for parsing evm.Call transactions using the contract
  -- abi when available.
  abi_parsed_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX ix_runtime_transactions_tx_hash ON chain.runtime_transactions USING hash (tx_hash);
CREATE INDEX ix_runtime_transactions_tx_eth_hash ON chain.runtime_transactions USING hash (tx_eth_hash);
CREATE INDEX ix_runtime_transactions_timestamp ON chain.runtime_transactions (runtime, timestamp);
CREATE INDEX ix_runtime_transactions_to ON chain.runtime_transactions(runtime, "to");
CREATE INDEX ix_runtime_transactions_to_abi_parsed_at ON chain.runtime_transactions (runtime, "to", abi_parsed_at);

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
  tx_eth_hash HEX64,
  timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
  
  -- TODO: add link to openapi spec section with runtime event types.
  type TEXT NOT NULL,
  -- The raw event, as returned by the oasis-sdk runtime client.
  body JSONB NOT NULL,
  related_accounts TEXT[],

  -- The events of type `evm.log` are further parsed into known event types, e.g. (ERC20) Transfer,
  -- to populate the `evm_log_name`, `evm_log_params`, and `evm_log_signature` fields.
  -- These fields are populated only when the ABI for the event is known. Typically, this is the
  -- case for events emitted by verified contracts.
  evm_log_name TEXT,
  evm_log_params JSONB,
  evm_log_signature BYTEA CHECK (octet_length(evm_log_signature) = 32),

  -- Internal tracking for parsing evm.Call events using the contract
  -- abi when available.
  abi_parsed_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX ix_runtime_events_round ON chain.runtime_events(runtime, round);  -- for sorting by round, when there are no filters applied
CREATE INDEX ix_runtime_events_tx_hash ON chain.runtime_events USING hash (tx_hash);
CREATE INDEX ix_runtime_events_tx_eth_hash ON chain.runtime_events USING hash (tx_eth_hash);
CREATE INDEX ix_runtime_events_related_accounts ON chain.runtime_events USING gin(related_accounts); -- for fetching account activity for a given account
CREATE INDEX ix_runtime_events_evm_log_signature ON chain.runtime_events(runtime, evm_log_signature, round); -- for fetching a certain event type, eg Transfers
CREATE INDEX ix_runtime_events_evm_log_params ON chain.runtime_events USING gin(evm_log_params);
CREATE INDEX ix_runtime_events_type ON chain.runtime_events (runtime, type);

CREATE TABLE chain.runtime_accounts
(
    runtime runtime NOT NULL,
    address oasis_addr NOT NULL,
    PRIMARY KEY (runtime, address),

    num_txs UINT63 NOT NULL DEFAULT 0,
    -- Total gas used by all txs addressed to this account. Primarily meaningful for accounts that are contracts.
    gas_for_calling UINT63 NOT NULL DEFAULT 0 -- gas used by txs sent to this address
);

-- Oasis addresses are derived from a derivation "context" and a piece of
-- data, such as an ed25519 public key or an Ethereum address. The derivation
-- is one-way, so you'd have to look up the address in this table and see if
-- we've encountered the preimage before to find out what the address was
-- derived from.
--
-- If you need to go the other way, from context + data to address, you'd 
-- normally just run the derivation. See oasis-core/go/common/crypto/address/address.go
-- for details. Consider inserting the preimage here if you're ingesting new 
-- blockchain data.
--
-- However, we do provide an index going the other way because certain queries 
-- require computing the derivation within Postgres and implementing/importing 
-- the right hash function will take some work.
-- TODO: import keccak hash into postgres.
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
CREATE INDEX ix_address_preimages_address_data ON chain.address_preimages (address_data) 
    WHERE context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND context_version = 0;

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
  PRIMARY KEY (runtime, token_address),
  token_type INTEGER, -- 0 = unsupported, X = ERC-X; full spec at https://github.com/oasisprotocol/nexus/blob/v0.0.16/analyzer/runtime/evm.go#L21
  token_name TEXT,
  symbol TEXT,
  decimals INTEGER,
  -- NOT an uint because a non-conforming token contract could issue a fake burn event,
  -- causing a negative dead-reckoned total_supply.
  total_supply NUMERIC(1000,0),
  
  num_transfers UINT63 NOT NULL DEFAULT 0,
  
  -- Block analyzer bumps this when it sees the mutable fields of the token
  -- change (e.g. total supply) based on dead reckoning.
  last_mutate_round UINT63 NOT NULL DEFAULT 0,
  
  -- Token analyzer bumps this when it downloads info about the token.
  last_download_round UINT63
);

CREATE TABLE analysis.evm_token_balances
(
  runtime runtime NOT NULL,
  -- This table is used to track balance querying primarily for EVM tokens (ERC-20, ERC-271, etc), but also for
  -- the runtime-native token; the latter is represented with a special addr: oasis1runt1menat1vet0ken0000000000000000000000
  token_address oasis_addr NOT NULL,
  account_address oasis_addr NOT NULL,
  PRIMARY KEY (runtime, token_address, account_address),
  
  last_mutate_round UINT63 NOT NULL,
  last_download_round UINT63
);
CREATE INDEX ix_evm_token_balance_analysis_stale ON analysis.evm_token_balances (runtime, token_address, account_address) WHERE last_download_round IS NULL OR last_mutate_round > last_download_round;

CREATE TABLE chain.evm_contracts
(
  runtime runtime NOT NULL,
  contract_address oasis_addr NOT NULL,
  PRIMARY KEY (runtime, contract_address),

  -- Can be null if the contract was created by another contract; eg through an evm.Call instead of a standard evm.Create. Tracing must be enabled to fill out this information.
  creation_tx HEX64,
  creation_bytecode BYTEA,
  runtime_bytecode BYTEA,

  -- Following fields are only filled out for contracts that have been verified.
  verification_info_downloaded_at TIMESTAMP WITH TIME ZONE, -- NULL for unverified contracts.
  abi JSONB,
   -- Contents of metadata.json, typically produced by the Solidity compiler.
  compilation_metadata JSONB,
  -- Each source file is a flat JSON object with keys "name", "content", "path", as returned by Sourcify.
  source_files JSONB CHECK (jsonb_typeof(source_files)='array')
);
CREATE INDEX ix_evm_contracts_unverified ON chain.evm_contracts (runtime) WHERE verification_info_downloaded_at IS NULL;

-- Used to keep track of potential contract addresses, and our progress in
-- downloading their runtime bytecode. ("Runtime" in the sense of ETH terminology
-- which talks about a contract's "runtime bytecode" as opposed to "creation bytecode".)
CREATE TABLE analysis.evm_contract_code (
    runtime runtime NOT NULL,
    contract_candidate oasis_addr NOT NULL,
    PRIMARY KEY (runtime, contract_candidate),
    -- Meaning of is_contract:
    --   TRUE:  downloaded runtime bytecode
    --   FALSE: download failed because `contract_candidate` is not a contract (= does not have code)
    --   NULL:  not yet attempted
    is_contract BOOLEAN
);
-- Allow the analyzer to quickly retrieve addresses that have not been downloaded yet.
CREATE INDEX ix_evm_contract_code_todo ON analysis.evm_contract_code (runtime, contract_candidate) WHERE is_contract IS NULL;

CREATE TABLE chain.evm_nfts (
    runtime runtime NOT NULL,
    token_address oasis_addr NOT NULL,
    nft_id uint_numeric NOT NULL,
    PRIMARY KEY (runtime, token_address, nft_id),

    last_want_download_round UINT63 NOT NULL,
    last_download_round UINT63,

    owner oasis_addr,
    num_transfers INT NOT NULL DEFAULT 0,
    metadata JSONB,
    metadata_uri TEXT,
    metadata_accessed TIMESTAMP,
    name TEXT,
    description TEXT,
    image TEXT
);
CREATE INDEX ix_evm_nfts_stale ON chain.evm_nfts (runtime, token_address, nft_id) WHERE last_download_round IS NULL OR last_want_download_round > last_download_round;
CREATE INDEX ix_evm_nfts_owner ON chain.evm_nfts (runtime, owner, token_address, nft_id);


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
CREATE INDEX ix_runtime_transfers_sender ON chain.runtime_transfers(runtime, sender);
CREATE INDEX ix_runtime_transfers_receiver ON chain.runtime_transfers(runtime, receiver);

-- -- -- -- -- -- -- -- -- -- -- -- -- Module consensusaccounts -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --

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
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;
GRANT SELECT ON ALL TABLES IN SCHEMA analysis TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA analysis TO PUBLIC;

COMMIT;
