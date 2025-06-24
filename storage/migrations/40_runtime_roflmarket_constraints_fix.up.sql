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

-- Add new FKs with provider included.
ALTER TABLE chain.roflmarket_instances
  ADD CONSTRAINT roflmarket_instances_runtime_provider_offer_id_fkey
  FOREIGN KEY (runtime, provider, offer_id)
  REFERENCES chain.roflmarket_offers(runtime, provider, id)
  DEFERRABLE INITIALLY DEFERRED;

-- Drop unneded index with the new PKs.
DROP INDEX IF EXISTS chain.ix_roflmarket_offers_provider;
DROP INDEX IF EXISTS chain.ix_roflmarket_instances_provider;

COMMIT;
