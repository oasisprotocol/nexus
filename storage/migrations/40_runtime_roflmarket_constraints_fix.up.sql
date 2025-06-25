BEGIN;

-- Drop existing invalid FKs on roflmarket_instances.
ALTER TABLE chain.roflmarket_instances
  DROP CONSTRAINT roflmarket_instances_runtime_offer_id_fkey;

-- Drop existing invalid PKs on roflmarket_offers and roflmarket_instances.
ALTER TABLE chain.roflmarket_offers
  DROP CONSTRAINT roflmarket_offers_pkey;

ALTER TABLE chain.roflmarket_instances
  DROP CONSTRAINT roflmarket_instances_pkey;

-- Add new PKs with provider included.
ALTER TABLE chain.roflmarket_offers
  ADD PRIMARY KEY (runtime, provider, id);

ALTER TABLE chain.roflmarket_instances
  ADD PRIMARY KEY (runtime, provider, id);

-- Drop unneded index with the new PKs.
DROP INDEX IF EXISTS chain.ix_roflmarket_offers_provider;
DROP INDEX IF EXISTS chain.ix_roflmarket_instances_provider;

-- Ensure refresh of providers.
UPDATE chain.roflmarket_providers
  SET last_processed_round = NULL;

COMMIT;
