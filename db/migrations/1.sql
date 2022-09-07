CREATE TABLE block_extra (
    chain_alias VARCHAR(32),
    height BIGINT,
    b_hash CHAR(64),
    gas_used BIGINT,
    size INTEGER
);
CREATE UNIQUE INDEX block_extra_chain_alias_height ON block_extra (chain_alias, height);
CREATE UNIQUE INDEX block_extra_hash ON block_extra (b_hash);

CREATE TABLE transaction_extra (
    chain_alias VARCHAR(32),
    height BIGINT,
    tx_index INTEGER,
    tx_hash CHAR(64),
    tx_from VARCHAR(46),
    nonce INTEGER
);
CREATE UNIQUE INDEX transaction_extra_chain_alias_height_index ON transaction_extra (chain_alias, height, tx_index);
CREATE INDEX transaction_extra_chain_alias_from_nonce ON transaction_extra (chain_alias, tx_from, nonce);
CREATE INDEX transaction_extra_hash ON transaction_extra (tx_hash);

CREATE TABLE related_transaction (
    chain_alias VARCHAR(32),
    account_address VARCHAR(46),
    tx_height BIGINT,
    tx_index INTEGER
);
CREATE INDEX related_transaction_chain_alias_account_address ON related_transaction (chain_alias, account_address);

CREATE TABLE progress (
    chain_alias VARCHAR(32) PRIMARY KEY,
    first_unscanned_height BIGINT
);
INSERT INTO progress (chain_alias, first_unscanned_height) VALUES ('mainnet_emerald', 2767126);
