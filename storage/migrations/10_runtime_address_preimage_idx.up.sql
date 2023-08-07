BEGIN;

-- Note: We use `IF NOT EXISTS` because the index has already been added manually in some places.
CREATE INDEX IF NOT EXISTS ix_address_preimages_address_data ON chain.address_preimages (address_data) 
    WHERE context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND 
        context_version = 0;

COMMIT;
