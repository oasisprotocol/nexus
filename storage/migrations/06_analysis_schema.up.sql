-- Moves analysis tables (= those that keep track of analyzers' internal state/progess) to the new `analysis` schema.

ALTER TABLE chain.processed_blocks SET SCHEMA analysis;

ALTER TABLE chain.evm_token_analysis SET SCHEMA analysis;
ALTER TABLE analysis.evm_token_analysis RENAME TO evm_tokens;

ALTER TABLE chain.evm_token_balance_analysis SET SCHEMA analysis;
ALTER TABLE analysis.evm_token_balance_analysis RENAME TO evm_token_balances;
