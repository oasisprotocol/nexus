CREATE TABLE chain.evm_nfts (
    runtime runtime NOT NULL,
    token_address oasis_addr NOT NULL,
    nft_id uint_numeric NOT NULL,
    PRIMARY KEY (runtime, token_address, nft_id),

    last_want_download_round UINT63 NOT NULL,
    last_download_round UINT63,

    metadata_uri TEXT,
    metadata_accessed TIMESTAMP,
    name TEXT,
    description TEXT,
    image TEXT
);
CREATE INDEX ix_evm_nfts_stale ON chain.evm_nfts (runtime, token_address, nft_id) WHERE last_download_round IS NULL OR last_want_download_round > last_download_round;
