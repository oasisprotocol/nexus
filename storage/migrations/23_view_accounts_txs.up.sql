-- Initialization of materialized views for efficient querying of Nexus data.

BEGIN;

CREATE MATERIALIZED VIEW views.accounts_tx_counts AS
    WITH
        -- Get the block at which the view was computed.
        max_block_height AS (
            SELECT MAX(height) AS height
            FROM chain.blocks
        )
    SELECT
        account_address as address,
        COUNT(*) AS tx_count,
        (SELECT height FROM max_block_height) AS computed_height
    FROM
        chain.accounts_related_transactions
    GROUP BY account_address;
CREATE UNIQUE INDEX ix_views_accounts_tx_counts ON views.accounts_tx_counts (address);

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA views TO PUBLIC;

COMMIT;
