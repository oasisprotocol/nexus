-- Add a primary key on debonding delegations. This is needed for single
-- deletions.
ALTER TABLE oasis_3.debonding_delegations ADD COLUMN id bigserial PRIMARY KEY;
