BEGIN;

-- Add indexes for evm_tokens token names and symbols for search.
CREATE INDEX ix_evm_tokens_name ON chain.evm_tokens USING GIST (token_name gist_trgm_ops);
CREATE INDEX ix_evm_tokens_symbol ON chain.evm_tokens USING GIST (symbol gist_trgm_ops);

COMMIT;
