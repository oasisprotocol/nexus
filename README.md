# Oasis Indexer

[![ci-lint](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-lint.yaml/badge.svg)](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-lint.yaml)
[![ci-test](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-test.yaml/badge.svg)](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-test.yaml)

The official indexer for the Oasis Network.

## Docker Development

You can create and run the Oasis Indexer with [`docker compose`](https://docs.docker.com/compose/).
Keep reading to get started, or take a look at our [Docker docs](docker/README.md) for more detail.

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
$ make start-docker
```

The analyzer will run DB migrations on start (i.e. create empty tables) based on files in `storage/migrations`.

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
You will need to set the Unix socket in the `config/local-dev.yaml` file while running an instance of the Oasis Indexer.
For example, this will be `unix:/node/data/internal.sock` in Docker.

Note: A newly created node takes a while to fully sync with the network.
The Oasis team has a node that is ready for internal use;
if you are a member of the team, ask around to use it and save time.

### Database

You will need to run a local [PostgreSQL DB](https://www.postgresql.org/).

For example, you can start a local [Docker](https://hub.docker.com/_/postgres) instance of Postgres with:
```
make postgres
```
and later browse the DB with
```
make psql
```

### Indexer

You should be able to `make oasis-indexer` and run `./oasis-indexer --config config/local-dev.yml` from the repository root.
This will start the entire indexer, but you can start each of its constituent services independently as well.
See `./oasis-indexer --help` for more details.

Once the indexer has started, you can query the Oasis Indexer API
```sh
$ curl -X GET http://localhost:8008/v1
```
