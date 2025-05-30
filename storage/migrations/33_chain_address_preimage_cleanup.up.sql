BEGIN;

-- There could potentially be some invalid address preimages in the DB,
-- due to a bug which was fixed in: https://github.com/oasisprotocol/nexus/pull/1023
DELETE FROM chain.address_preimages
WHERE context_identifier = 'oasis-core/address: staking'
  AND context_version = 0
  AND length(address_data) = 21;

COMMIT;
