# Migrations

This directory contains migrations for the Oasis Indexer target model,
i.e. processed blocks, transactions, and events from the consensus layer and
each respective runtime.

## Naming Convention

Migration filenames should be of the form `<id>-<name>.sql`, where `<id>` is the 4-digit index of the migration and `<name>` is a concise descriptor for the migration. For example, `0000-init.sql`.

We add the extra convention that migrations tied to major network upgrades specify `<name>` as `<chain-id>-init.sql`.
For example, `0001-oasis-3-init.sql` might encode initialization for the [Damask upgrade](https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2022-04-11).

## Generation

The Oasis Indexer supports a migration generator for truncating existing state tables and inserting new state from a genesis file. Example usage is as follows:

```sh
oasis-indexer generate \
  --generator.genesis_file config/genesis.json \
  --generator.migration_file ./storage/migrations/0000-example-migration.sql
```
