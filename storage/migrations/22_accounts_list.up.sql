BEGIN;

CREATE SCHEMA IF NOT EXISTS views;
GRANT USAGE ON SCHEMA views TO PUBLIC;

CREATE MATERIALIZED VIEW views.accounts_list AS
	WITH
	-- Compute active delegations balances for all accounts with delegations.
	delegation_balances AS (
		SELECT
			d.delegator,
			COALESCE(ROUND(SUM(d.shares * a.escrow_balance_active / NULLIF(a.escrow_total_shares_active, 0))), 0) AS delegations_balance
		FROM chain.delegations d
		JOIN chain.accounts a ON d.delegatee = a.address
		GROUP BY d.delegator
	),
	-- Compute debonding delegations balances for all accounts with debonding delegations.
	debonding_balances AS (
		SELECT
			dd.delegator,
			COALESCE(ROUND(SUM(dd.shares * a.escrow_balance_debonding / NULLIF(a.escrow_total_shares_debonding, 0))), 0) AS debonding_delegations_balance
		FROM chain.debonding_delegations dd
		JOIN chain.accounts a ON dd.delegatee = a.address
		GROUP BY dd.delegator
	),
	-- Get the block at which the view was computed.
    max_block_height AS (
        SELECT MAX(height) AS height
        FROM chain.blocks
    )
	SELECT
		a.address,
		a.nonce,
		a.general_balance,
		a.escrow_balance_active,
		a.escrow_balance_debonding,
		COALESCE(ad.delegations_balance, 0) AS delegations_balance,
		COALESCE(dd.debonding_delegations_balance, 0) AS debonding_delegations_balance,
		a.general_balance + COALESCE(ad.delegations_balance, 0) + COALESCE(dd.debonding_delegations_balance, 0) AS total_balance,
		(SELECT height FROM max_block_height) AS computed_height
	FROM
		chain.accounts a
	LEFT JOIN
		delegation_balances ad ON a.address = ad.delegator
	LEFT JOIN
		debonding_balances dd ON a.address = dd.delegator;
CREATE UNIQUE INDEX ix_views_accounts_list_address ON views.accounts_list (address); -- A unique index is required for CONCURRENTLY refreshing the view.
CREATE INDEX ix_views_account_list_total_balance_address ON views.accounts_list (total_balance DESC, address); -- Index for sorting by total balance.

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA views TO PUBLIC;

COMMIT;
