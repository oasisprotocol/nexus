CREATE TABLE block_extra (
    chain_alias VARCHAR(32),
    round BIGINT,
    gas_used BIGINT,
    size INTEGER
);
CREATE UNIQUE INDEX block_extra_chain_alias_round ON block_extra (chain_alias, round);
