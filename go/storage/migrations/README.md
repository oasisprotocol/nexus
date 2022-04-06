# Migrations

This directory contains migrations for the Oasis Indexer target model,
i.e. processed blocks, transactions, and events from the consensus layer and
each respective runtime.

## Naming Convention

Migration filenames should be of the form `<id>-<name>.sql`, where `<id>` is the 4-digit index of the migration and `<name>` is a concise descriptor for the migration. For example, `0000-init.sql`.

We add the extra convention that migrations tied to major network upgrades specify `<name>` as `<chain-id>-init.sql`.
For example, `0003-oasis-2-init.sql` might encode initialization for the [Cobalt upgrade](https://docs.oasis.dev/general/mainnet/cobalt-upgrade).
