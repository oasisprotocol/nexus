CREATE TABLE block_extra (
    chain_alias VARCHAR(32),
    height BIGINT,
    gas_used BIGINT,
    size INTEGER
);
CREATE UNIQUE INDEX block_extra_chain_alias_height ON block_extra (chain_alias, height);

CREATE TABLE progress (
    chain_alias VARCHAR(32) PRIMARY KEY,
    first_unscanned_height BIGINT
);
INSERT INTO progress (chain_alias, first_unscanned_height) VALUES ('mainnet_emerald', 2715686);
