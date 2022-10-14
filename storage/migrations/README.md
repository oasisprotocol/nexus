# Migrations

This directory contains migrations for the Oasis Indexer target model,
i.e. processed blocks, transactions, and events from the consensus layer and
each respective runtime.

## Naming Convention

Migration filenames should follow the [convention](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md) `<id>_<name>.up.sql`, where `<id>` is the 4-digit index of the migration and `<name>` is a concise descriptor for the migration. For example, `0000_init.up.sql`.

We add the extra convention that migrations tied to major network upgrades specify `<name>` as `<chain_id>_init.sql`.
For example, `0001_oasis_3_init.sql` might encode initialization for the [Damask upgrade](https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2022-04-11).

We do not expect to need the [down](https://github.com/golang-migrate/migrate/blob/master/FAQ.md#why-two-separate-files-up-and-down-for-a-migration) migrations.

