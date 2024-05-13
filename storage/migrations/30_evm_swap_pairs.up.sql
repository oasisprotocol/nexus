BEGIN;

CREATE TABLE chain.evm_swap_pair_creations (
    runtime runtime NOT NULL,
    factory_address oasis_addr NOT NULL,
    token0_address oasis_addr NOT NULL,
    token1_address oasis_addr NOT NULL,
    pair_address oasis_addr NOT NULL,
    create_round UINT63 NOT NULL,
    PRIMARY KEY (runtime, factory_address, token0_address, token1_address)
);

CREATE TABLE chain.evm_swap_pairs (
    runtime runtime NOT NULL,
    pair_address oasis_addr NOT NULL,
    PRIMARY KEY (runtime, pair_address),
    reserve0 uint_numeric NOT NULL,
    reserve1 uint_numeric NOT NULL,
    last_sync_round UINT63 NOT NULL
);

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
