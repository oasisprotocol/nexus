# Oasis Indexer

The Oasis Indexer is an data processing pipeline for extracting and analyzing raw data from the Oasis Network.
It enables fast, easy, and reliable querying of network state without requiring node setup and maintenance.

See instructions below for running your own indexer [locally](#local-development) or with [docker](#docker-development).

## Code Organization

The code is organized into the following packages:

- `analyzer` contains all network analyzers, as well as interface definitions which  they must support.
- `api` contains API definitions and implementation for all exposed endpoints by the `oasis-indexer`.
- `config` contains configuration files for settings tuning.
- `docker` contains tooling for local development and containerization.
- `log` contains utilities for structured logging.
- `metrics` contains utilities for structured instrumentation.
- `oasis-indexer` contains the indexer entrypoint, including code for all generated binaries and their configuration parameters.
- `storage` contains the storage interfaces and database logic for source and destination storage within the indexer ETL system.

## Local Development

### Oasis Node

You will need to run a local [node](https://docs.oasis.dev/general/run-a-node/set-up-your-node/run-non-validator) for development purposes.
You will need the Unix socket for the `network.yaml` file while running an analyzer service.
For example, this will be `unix:/node/data/internal.sock` in Docker.

### Cockroach DB

You will need to run a local [Cockroach DB](https://www.cockroachlabs.com/docs/).

### Indexer

You should be able to `go build` the indexer and run `./oasis-indexer [cmd]` from the repo top
level directory.

## Docker Development

You can create and run a Dockerized version of the Oasis Indexer as follows.

**Configuration**

Download the current network's [genesis document](https://docs.oasis.dev/oasis-core/consensus/genesis)
to the `docker/node/etc` directory. You will need this to run the Oasis Node container.

**Build**

From within the `docker` directory, you can run:
```sh
$ make docker-build
```

**Run**
From within the `docker` directory, you can run:
```sh
$ make docker-up
```
