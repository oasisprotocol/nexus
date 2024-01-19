-- Helper functions for manipulating oasis data types.
BEGIN;

-------------------------------------
-- Convenience functions for converting between eth-style and oasis-style addresses
-- via lookups in the address_preimages table.

-- Convenience function for retrieving the ethereum address associated with a given oasis address.
CREATE OR REPLACE FUNCTION public.eth_preimage(addr oasis_addr)
RETURNS BYTEA
LANGUAGE plpgsql
AS $$
BEGIN
DECLARE
    result BYTEA;
BEGIN
    SELECT pre.address_data
    INTO result
    FROM chain.address_preimages pre
    WHERE pre.address = addr
    AND pre.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
    AND pre.context_version = 0;
    RETURN result;
END;
END;
$$;

CREATE OR REPLACE FUNCTION public.derive_oasis_addr(eth_addr bytea)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
BEGIN
DECLARE
    result TEXT;
BEGIN
    -- Check if the bytea data is exactly 20 bytes in length; this prevents accidentally accepting strings that are hex-encodings of the actual address, or similar.
    IF length(eth_addr) <> 20 THEN
        RAISE EXCEPTION 'Input address must be a bytea and exactly 20 bytes in length';
    END IF;

    -- Look up the oasis-style address derived from evs.body.address.
    -- The derivation is just a keccak hash and we could theoretically compute it instead of looking it up,
    -- but the right hash function will only be easily available in postgres once openssl 3.2 is released
    -- (see https://github.com/openssl/openssl/issues/19304) and bundled into a postgres docker image, probably
    -- in early 2024.
    SELECT pre.address
    INTO result
    FROM chain.address_preimages pre
    WHERE pre.address_data = eth_addr
    AND pre.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
    AND pre.context_version = 0;
    RETURN result;
END;
END;
$$;


-------------------------------------
COMMIT;
