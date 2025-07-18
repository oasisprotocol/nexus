package nebyprices

// nolint: gosec // G101: Potential hardcoded credentials.
const tokenNebyDerivedPriceUpsert = `
INSERT INTO chain.evm_tokens (runtime, token_address, neby_derived_price)
	SELECT
		$1::runtime, derive_oasis_addr($2), $3
	WHERE
		derive_oasis_addr($2) IS NOT NULL
	ON CONFLICT (runtime, token_address) DO UPDATE
	SET
	neby_derived_price = excluded.neby_derived_price`
