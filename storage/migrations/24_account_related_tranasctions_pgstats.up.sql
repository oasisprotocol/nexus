BEGIN;

-- https://github.com/oasisprotocol/nexus/issues/993#issuecomment-2863509435
ALTER TABLE chain.accounts_related_transactions ALTER COLUMN tx_block SET STATISTICS 200; -- By default read from `default_statistics_target` which is 100, unless changed.

COMMIT;
