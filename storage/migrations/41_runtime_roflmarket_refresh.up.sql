BEGIN;

-- Ensure refresh of providers with the fixed code.
UPDATE chain.roflmarket_providers
  SET last_processed_round = NULL;

COMMIT;
