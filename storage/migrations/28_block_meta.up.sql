BEGIN;

CREATE TABLE chain.block_nodes
(
    id      TEXT PRIMARY KEY, -- The consensus address of the signer/proposer node.
    address oasis_addr NOT NULL UNIQUE, -- The oasis address of the node (can't be directly derived from the consensus address).
    CONSTRAINT address_combo UNIQUE (id, address)
);

-- ALTER TABLE chain.nodes
--     ADD COLUMN consensus_pubkey_address TEXT;

-- CREATE INDEX ix_nodes_consensus_pubkey_address ON chain.nodes (consensus_pubkey_address);

ALTER TABLE chain.blocks
    ALTER COLUMN block_hash DROP NOT NULL,
    ALTER COLUMN time DROP NOT NULL,
    ALTER COLUMN num_txs DROP NOT NULL,
    ALTER COLUMN namespace DROP NOT NULL,
    ALTER COLUMN version DROP NOT NULL,
    ALTER COLUMN state_root DROP NOT NULL,
    ADD COLUMN proposer_node TEXT REFERENCES chain.block_nodes(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE chain.block_signer_links
(
    id     SERIAL PRIMARY KEY,
    block  UINT63 NOT NULL REFERENCES chain.blocks(height) DEFERRABLE INITIALLY DEFERRED,
    signer TEXT NOT NULL REFERENCES chain.block_nodes(height) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT unique_block_signer UNIQUE (block, signer)
);

COMMIT;
