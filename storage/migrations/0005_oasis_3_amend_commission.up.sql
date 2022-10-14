CREATE TABLE IF NOT EXISTS oasis_3.commissions
(
  address  TEXT PRIMARY KEY NOT NULL REFERENCES oasis_3.accounts(address),
  schedule JSON
);

-- The rest of this "migration" was really an import of genesis state.
-- Leaving the file present but empty to avoid breaking the sequential migration numbering.
