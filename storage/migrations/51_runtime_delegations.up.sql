BEGIN;

-- Runtime delegations are layered on top of consensus-layer delegations.
-- From the perspective of the consensus layer, a single "runtime address" delegates to a consensus account.
-- However, within the runtime, this delegation is further subdivided and tracked per individual runtime address.
-- This allows the runtime to maintain its own delegation accounting, independent of how the consensus views the delegation.
-- This table tracks the runtime-level delegation accounting.
-- The denomination of the shares is also the consensus denomination, so we don't store it here.
CREATE TABLE chain.runtime_accounts_delegations
(
    runtime runtime NOT NULL,

    -- Delegator is the runtime account that is delegating.
    delegator oasis_addr NOT NULL,
    -- Delegatee is a consensus account delegated to.
    delegatee oasis_addr NOT NULL, -- This could be FK chain.accounts(address), but we don't want runtime-consensus references at the moment (cannot test runtime analyzer without consensus).
    shares    UINT_NUMERIC NOT NULL,

    PRIMARY KEY (runtime, delegator, delegatee),
    FOREIGN KEY (runtime, delegator) REFERENCES chain.runtime_accounts(runtime, address) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE chain.runtime_accounts_debonding_delegations
(
    runtime runtime NOT NULL,

    -- Delegator is the runtime account that is delegating.
    delegator oasis_addr NOT NULL,
    -- Delegatee is a consensus account delegated to.
    delegatee oasis_addr NOT NULL,  -- This could be FK chain.accounts(address), but we don't want runtime-consensus references at the moment (cannot test runtime analyzer without consensus).
    shares    UINT_NUMERIC NOT NULL,
    debond_end UINT63 NOT NULL,

    PRIMARY KEY (runtime, delegator, delegatee, debond_end),
    FOREIGN KEY (runtime, delegator) REFERENCES chain.runtime_accounts(runtime, address) DEFERRABLE INITIALLY DEFERRED
);

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
