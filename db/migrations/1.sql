CREATE TABLE block_extra (
    chain_alias VARCHAR(32),
    height BIGINT,
    b_hash CHAR(64),
    gas_used BIGINT,
    size INTEGER,
    PRIMARY KEY (chain_alias, height)
);
CREATE UNIQUE INDEX block_extra_hash ON block_extra (b_hash);

CREATE TABLE transaction_extra (
    chain_alias VARCHAR(32),
    height BIGINT,
    tx_index INTEGER,
    tx_hash CHAR(64),
    eth_hash CHAR(64),
    PRIMARY KEY (chain_alias, height, tx_index)
);
CREATE INDEX transaction_extra_hash ON transaction_extra (tx_hash);
CREATE INDEX transaction_extra_eth_hash ON transaction_extra (eth_hash);

CREATE TABLE transaction_signer (
    chain_alias VARCHAR(32),
    height BIGINT,
    tx_index INTEGER,
    signer_index INTEGER,
    addr VARCHAR(46),
    nonce INTEGER,
    PRIMARY KEY (chain_alias, height, tx_index, signer_index)
);
CREATE INDEX transaction_signer_chain_alias_signer_addr_nonce ON transaction_signer (chain_alias, addr, nonce);

CREATE TABLE related_transaction (
    chain_alias VARCHAR(32),
    account_address VARCHAR(46),
    tx_height BIGINT,
    tx_index INTEGER
);
CREATE INDEX related_transaction_chain_alias_account_address ON related_transaction (chain_alias, account_address);

CREATE TABLE address_preimage (
    address VARCHAR(46) PRIMARY KEY,
    context_identifier VARCHAR(64),
    context_version INTEGER,
    addr_data bytea
);

CREATE TABLE progress (
    chain_alias VARCHAR(32) PRIMARY KEY,
    first_unscanned_height BIGINT
);
INSERT INTO progress (chain_alias, first_unscanned_height) VALUES ('mainnet_emerald', 2880214);
