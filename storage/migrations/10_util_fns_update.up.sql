BEGIN;

-- Marks public.eth_preimage as STABLE.
-- This enables the function to be used in index expressions.
CREATE OR REPLACE FUNCTION public.eth_preimage(addr oasis_addr)
RETURNS BYTEA
LANGUAGE plpgsql
STABLE
AS $$
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
$$;

-- Marks public.derive_oasis_addr as STABLE.
-- This enables the function to be used in index expressions.
CREATE OR REPLACE FUNCTION public.derive_oasis_addr(eth_addr bytea)
RETURNS TEXT
LANGUAGE plpgsql
STABLE
AS $$
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
    -- Once the above is computable, the function can be changed to IMMUTABLE.
    SELECT pre.address
    INTO result
    FROM chain.address_preimages pre
    WHERE pre.address_data = eth_addr
    AND pre.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
    AND pre.context_version = 0;
    RETURN result;
END;
$$;

COMMIT;
