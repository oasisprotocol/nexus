BEGIN;

ALTER TABLE chain.entities
    ADD COLUMN logo_url TEXT;

COMMIT;
