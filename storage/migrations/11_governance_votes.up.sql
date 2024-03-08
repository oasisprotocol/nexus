BEGIN;

ALTER TABLE chain.votes
    ADD COLUMN height UINT63; -- Can be NULL; when the vote comes from a genesis file, height is unknown.

COMMIT;
