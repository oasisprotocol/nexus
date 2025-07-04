# Oasis Nexus

[![ci-lint](https://github.com/oasisprotocol/nexus/actions/workflows/ci-lint.yaml/badge.svg)](https://github.com/oasisprotocol/nexus/actions/workflows/ci-lint.yaml)
[![ci-test](https://github.com/oasisprotocol/nexus/actions/workflows/ci-test.yaml/badge.svg)](https://github.com/oasisprotocol/nexus/actions/workflows/ci-test.yaml)

The official indexer for the Oasis Network. Nexus continuously fetches
blockchain data from one or more oasis nodes and related sources
([Sourcify](sourcify.dev),
[Oasis Metadata Registry](https://github.com/oasisprotocol/metadata-registry),
...), parses the data and stores it into a heavily indexed SQL database, and
provides a JSON-based web API to access the data.

Nexus aims to serve as the backend for explorers and wallets, notably the
official [Oasis Explorer](https://github.com/oasisprotocol/explorer/) and
[Oasis Wallet](https://github.com/oasisprotocol/oasis-wallet-web).

## Docker Development

You can build and run Oasis Nexus with
[`docker compose`](https://docs.docker.com/compose/). Keep reading to get
started, or take a look at our [Docker docs](docker/README.md) for more detail.

### Configuration

Download the current network's
[genesis document](https://docs.oasis.dev/oasis-core/consensus/genesis) to the
`docker/node/etc` directory. You will need this to run the Oasis Node container.

### Build

From the repository root, you can run:

```sh
make docker
```

### Run

From the repository root, you can run:

```sh
make start-docker
```

The analyzer will run DB migrations on start (i.e. create empty tables) based on
files in `storage/migrations`.

### Query

Now you can query the Oasis Nexus API

```sh
curl -X GET http://0.0.0.0:8008/v1
```

For a full list of endpoints see our
[API docs](https://github.com/oasisprotocol/nexus/blob/main/api/README.md).

## Local Development

Below are instructions for running Oasis Nexus locally, without Docker.

### Oasis Node

You will need to run a local
[node](https://docs.oasis.io/node/run-your-node/non-validator-node/) for
development purposes. You will need to set the Unix socket in the
`config/local-dev.yaml` file while running an instance of Oasis Nexus. For
example, this will be `unix:/node/data/internal.sock` in Docker.

**Note:** A newly created node takes a while to fully sync with the network. The
Oasis team has a node that is ready for internal use; if you are a member of the
team, ask around to use it and save time.

### Database

You will need to run a local [PostgreSQL DB](https://www.postgresql.org/).

For example, you can start a local [Docker](https://hub.docker.com/_/postgres)
instance of Postgres with:

```
make postgres
```

and later browse the DB with

```
make psql
```

### Nexus

You should be able to `make nexus` and run
`./nexus --config config/local-dev.yml` from the repository root. This will
start the analyzers and the HTTP server, but you can start each of the
constituent services independently as well. See `./nexus --help` for more
details.

Once Nexus has started, you can query the Oasis Nexus API

```sh
curl -X GET http://localhost:8008/v1
```

**Debugging note**: A lot of indexing happens when parsing the genesis data. To
see what SQL statements genesis is converted into, run `nexus` with
`NEXUS_DUMP_GENESIS_SQL=/tmp/genesis.sql`. The SQL will be written to the
indicated file, provided that genesis hasn't been parsed into the DB yet. The
easiest way to achieve the latter is to wipe the DB.

### Code Quality Tools / Dependencies

Here are our recommendations for getting the tools that `make lint` and
`make fmt` use. None of these are strictly needed to compile Nexus or even to
create a PR, but without them, you're at the mercy of CI.

- **golangci-lint**:
  [use their `curl | sh` installer](https://golangci-lint.run/usage/install/).
  They explain that the tool is in dependency hell and any more structured
  distribution of it might not work.

  ```sh
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin v2.2.1
  ```

  - **gofumpt**: `go install mvdan.cc/gofumpt@latest`

    Note: CI uses gofumpt through golangci-lint, so if there's any discrepancy,
    look up what gofumpt version golangci-lint uses. Beware that we might not be
    on the latest golangci-lint either.

  - **goimports**: `go install golang.org/x/tools/cmd/goimports@latest`

    Note: CI uses goimports through golangci-lint, so if there's any
    discrepancy, look up what `golang.org/x/tools` version golangci-lint uses.
    Beware that we might not be on the latest golangci-lint either.

- **gitlint**: `pip install gitlint`

  For linting commit messages. Used by [git hooks](scripts/git-hooks) and
  `make lint-git`.

- **gh**: `brew install gh` or
  [see official instructions](https://github.com/cli/cli?tab=readme-ov-file#installation)
  for Linux. After installation, `gh auth login`.

  GitHub CLI. Used by [git hooks](scripts/git-hooks).

- **punch.py**: `pip install punch.py`

  Keeps track of the most recently released version.

- **prettier**: `npm install --save-dev --save-exact -g prettier`

  For rewrapping (and some other normalization?) of Markdown files (including
  changelogs) and commit messages. Used by [git hooks](scripts/git-hooks) and
  recommended as autoformatter in your text editor (setup not covered here).

## Name Origin

"Nexus" is a Latin word, meaning "connection or series of connections linking
two or more things". Similarly, Oasis Nexus connects off-chain products with the
Oasis blockchain.
