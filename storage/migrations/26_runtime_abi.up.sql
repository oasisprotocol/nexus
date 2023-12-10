BEGIN;

ALTER TABLE chain.runtime_transactions
    -- For evm.Call transactions, we store both the name of the function and
    -- the function parameters.
    ADD COLUMN evm_fn_name TEXT,
    -- The function parameter values. Refer to the abi to see the parameter
    -- names. Note that the parameters may be unnamed.
    ADD COLUMN evm_fn_params JSONB,
    -- Custom errors may be arbitrarily defined by the contract abi. This field
    -- stores the full abi-decoded error object. Note that the error name is 
    -- stored separately in the existing error_message column. For example, if we
    -- have an error like `InsufficientBalance{available: 4, required: 10}`.
    -- the error_message column would hold `InsufficientBalance`, and 
    -- the error_params column would store `{available: 4, required: 10}`.
    ADD COLUMN error_params JSONB,
    -- Internal tracking for parsing evm.Call transactions using the contract
    -- abi when available.
    ADD COLUMN abi_parsed_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX ix_runtime_transactions_to_abi_parsed_at ON chain.runtime_transactions (runtime, "to")
    WHERE abi_parsed_at IS NULL;

ALTER TABLE chain.runtime_events
    -- Internal tracking for parsing evm.Call transactions using the contract
    -- abi when available.
    ADD COLUMN abi_parsed_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX ix_runtime_events_type ON chain.runtime_events (runtime, type);

COMMIT;
