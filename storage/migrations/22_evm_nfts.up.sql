BEGIN;

ALTER TABLE chain.evm_nfts
    ADD COLUMN owner oasis_addr,
    ADD COLUMN num_transfers INT NOT NULL DEFAULT 0;

CREATE INDEX ix_evm_nfts_owner ON chain.evm_nfts (runtime, owner, token_address, nft_id);

-- Grant others read-only use.
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
