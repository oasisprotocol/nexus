# Oasis Indexer

The official indexer for the Oasis Network.

## Docker Development

You can create and run the Oasis Indexer with [`docker compose`](https://docs.docker.com/compose/).

**Configuration**

Download the current network's [genesis document](https://docs.oasis.dev/oasis-core/consensus/genesis)
to the `docker/node/etc` directory. You will need this to run the Oasis Node container.

**Build**

From the repository root, you can run:
```sh
$ make docker-build
```

**Run**
From the repository root, you can run:
```sh
$ make docker-up
```

The analyzer will run migrations automagically on start based on files in `storage/migrations`.

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

You should be able to `go build` the indexer and run `./oasis-indexer [cmd]` from the repo top
level directory.
