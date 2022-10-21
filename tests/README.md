# Oasis Indexer Tests

This directory contains tests for the Oasis Indexer. They have been organized as follows:

- `e2e/` contains tests that run against a fake blockchain (provided by `oasis-net-runner`) and a black-box indexer (i.e. only external APIs are accessed).
- `genesis/` contains tests that validate our database state is correct against `oasis-node` genesis state.
- `http/` contains tests that validate API endpoints behave as expected and return correct output.

## E2E Tests

To ensure that tests behave as expected, you should have Docker environment configured as per the [top-level README](../README.md#docker-development).

### Running e2e tests locally

For the canonical way to run these tests, see the [CI config](../../.github/workflows/ci-test.yaml).
Here are some useful one-liners derived from those commands, and adapted for running locally and repeatedly.

### Get docker containers with current code up&running
```sh
make docker &&
{ docker compose -f tests/e2e/docker-compose.e2e.yml down -t 0 || true; } &&
make clean-e2e &&
env HOST_UID=$(id -u) HOST_GID=$(id -g) docker compose -f tests/e2e/docker-compose.e2e.yml up -d
```

`make clean-e2e` wipes the state for `oasis-net-runner`; state left over from previous runs prevents the runner from starting. (Error: `failed to provision entity: oasis/entity: failed to create deterministic identity: signature/signer/file: key already exists`)

### Run the e2e tests
You can run tests from the `oasis-indexer` container:

```sh
docker exec oasis-indexer sh -c "cd /oasis-indexer && make test-e2e"
```

### Check indexer logs
```sh
docker logs oasis-indexer --since 2m -t | less
```
