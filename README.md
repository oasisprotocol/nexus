# Oasis Indexer

[![ci-lint](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-lint.yaml/badge.svg)](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-lint.yaml)
[![ci-test](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-test.yaml/badge.svg)](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-test.yaml)

The official indexer for the Oasis Network.

## Docker Development

You can create and run the Oasis Indexer with [`docker compose`](https://docs.docker.com/compose/).
Keep reading to get started, or take a look at our [Docker docs](https://github.com/oasislabs/oasis-indexer/blob/main/docker/README.md) for more detail.

**Configuration**

Download the current network's [genesis document](https://docs.oasis.dev/oasis-core/consensus/genesis)
to the `docker/node/etc` directory. You will need this to run the Oasis Node container.

**Build**

From the repository root, you can run:
```sh
$ make docker
```

**Run**
From the repository root, you can run:
```sh
$ make run
```

The analyzer will run migrations automagically on start based on files in `storage/migrations`.
See [Generating Migrations](#generating-migrations) for information on generating new migrations.

**Query**
Now you can query the Oasis Indexer API
```sh
$ curl -X GET http://0.0.0.0:8008/v1
```

For a full list of endpoints see our [API docs](https://github.com/oasislabs/oasis-indexer/blob/main/api/README.md).

## Local Development

Below are instructions for running the Oasis Indexer locally, without Docker.

### Oasis Node

You will need to run a local [node](https://docs.oasis.dev/general/run-a-node/set-up-your-node/run-non-validator) for development purposes.
You will need the Unix socket for the `network.yaml` file while running an analyzer service.
For example, this will be `unix:/node/data/internal.sock` in Docker.

### Database

You will need to run a local [PostgreSQL DB](https://www.postgresql.org/).

For example, a local [Docker](https://hub.docker.com/_/postgres) version would look like:
```
docker run
  --name postgres
  -p 5432:5432
  -e POSTGRES_USER=indexer
  -e POSTGRES_PASSWORD=password
  -e POSTGRES_DB=indexer
  -d
  postgres
```

### Indexer

You should be able to `make oasis-indexer` and run `./oasis-indexer [cmd]` from the repository root.

## Generating Migrations

The Oasis Indexer supports generating SQL migrations from a genesis document to initialize indexed state.
You can do so as follows:

```sh
oasis-indexer generate \
  --generator.genesis_file path/to/your/genesis.json
  --generator.migration_file storage/migrations/nnnn_example.up.sql
```

or directly from a running node

```sh
oasis-indexer generate \
  --generator.network_config_file path/to/your/config.yaml
  --generator.migration_file storage/migrations/nnnn_example.up.sql
```

See our [naming convention](https://github.com/oasislabs/oasis-indexer/blob/main/storage/migrations/README.md#naming-convention) for how to aptly name your migrations.
