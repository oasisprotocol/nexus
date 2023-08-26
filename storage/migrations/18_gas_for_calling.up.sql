BEGIN; 

-- Moves the chain.evm_contracts.gas_used column to chain.runtime_accounts.gas_for_calling

ALTER TABLE chain.runtime_accounts
    -- Total gas used by all txs addressed to this account. Primarily meaningful for accounts that are contracts.
    ADD COLUMN gas_for_calling UINT63 NOT NULL DEFAULT 0;

UPDATE chain.runtime_accounts as ra
    SET gas_for_calling = c.gas_used
    FROM chain.evm_contracts c
    WHERE c.runtime = ra.runtime AND
          c.contract_address = ra.address;

ALTER TABLE chain.evm_contracts
    DROP COLUMN gas_used;

COMMIT;
