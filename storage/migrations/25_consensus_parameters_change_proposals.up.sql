BEGIN;

ALTER TABLE chain.proposals
    ADD COLUMN parameters_change_module TEXT,
    ADD COLUMN parameters_change BYTEA;

COMMIT;
