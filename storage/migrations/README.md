# Migrations

This directory contains migrations for the Oasis Nexus target model, i.e.
processed blocks, transactions, and events from the consensus layer and each
respective runtime.

## Naming Convention

Migration filenames should follow the
[convention](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md)
`<id>_<name>.up.sql`, where `<id>` is the 2-digit index of the migration and
`<name>` is a concise descriptor for the migration. For example,
`00_init.up.sql`.

We do not expect to need the
[down](https://github.com/golang-migrate/migrate/blob/master/FAQ.md#why-two-separate-files-up-and-down-for-a-migration)
migrations.
