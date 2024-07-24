BEGIN;

ALTER TABLE chain.proposals
	ADD COLUMN title TEXT;

ALTER TABLE chain.proposals
	ADD COLUMN description TEXT;

COMMIT;
