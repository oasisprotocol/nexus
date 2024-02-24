BEGIN;

ALTER TABLE chain.nodes
    ADD COLUMN consensus_pubkey_address TEXT;

CREATE INDEX ix_nodes_consensus_pubkey_address ON chain.nodes (consensus_pubkey_address);

ALTER TABLE chain.blocks
    ALTER COLUMN block_hash DROP NOT NULL,
    ALTER COLUMN time DROP NOT NULL,
    ALTER COLUMN num_txs DROP NOT NULL,
    ALTER COLUMN namespace DROP NOT NULL,
    ALTER COLUMN version DROP NOT NULL,
    ALTER COLUMN state_root DROP NOT NULL,
    ADD COLUMN proposer_node_consensus_pubkey_address TEXT,
    ADD COLUMN signer_node_consensus_pubkey_addresses TEXT[];

COMMIT;
