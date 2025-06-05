BEGIN;

-- Fix previously double-hex encoded fields in roflmarket tables.

-- Providers: offers_next_id, instances_next_id.
UPDATE chain.roflmarket_providers
SET
  offers_next_id = decode(convert_from(offers_next_id, 'UTF8'), 'hex'),
  instances_next_id = decode(convert_from(instances_next_id, 'UTF8'), 'hex');

-- Offers: id.
UPDATE chain.roflmarket_offers
SET
  id = decode(convert_from(id, 'UTF8'), 'hex');

-- Instances: id, offer_id, cmd_next_id.
UPDATE chain.roflmarket_instances
SET
  id = decode(convert_from(id, 'UTF8'), 'hex'),
  offer_id = decode(convert_from(offer_id, 'UTF8'), 'hex'),
  cmd_next_id = decode(convert_from(cmd_next_id, 'UTF8'), 'hex');

COMMIT;
