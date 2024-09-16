BEGIN;

ALTER TABLE chain.blocks
    ADD COLUMN proposer_entity_id base64_ed25519_pubkey,
    ADD COLUMN signer_entity_ids base64_ed25519_pubkey[];

CREATE INDEX ix_blocks_proposer_entity_id ON chain.blocks (proposer_entity_id);
CREATE INDEX ix_blocks_signer_entity_ids ON chain.blocks USING gin(signer_entity_ids);

COMMIT;
