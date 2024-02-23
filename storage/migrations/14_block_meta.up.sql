BEGIN;

ALTER TABLE chain.nodes
    ADD COLUMN consensus_pubkey_address TEXT;

CREATE INDEX ix_nodes_consensus_pubkey_address ON chain.nodes (consensus_pubkey_address);

COMMIT;
