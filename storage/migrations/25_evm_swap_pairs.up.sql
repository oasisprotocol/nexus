BEGIN;

CREATE TABLE chain.evm_swap_pairs (
    runtime runtime NOT NULL,
    pair_address oasis_addr NOT NULL,
    factory_address oasis_addr,
    token0_address oasis_addr,
    token1_address oasis_addr,
    create_round UINT63,
    PRIMARY KEY (runtime, pair_address),
    reserve0 uint_numeric,
    reserve1 uint_numeric,
    last_sync_round UINT63
);

CREATE INDEX ix_evm_swap_pairs_factory_tokens
    ON chain.evm_swap_pairs(runtime, factory_address, token0_address, token1_address)
    WHERE
        factory_address IS NOT NULL AND
        token0_address IS NOT NULL AND
        token1_address IS NOT NULL;

COMMIT;
