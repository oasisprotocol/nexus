# End-to-end tests

These tests are intended to be run against a fake blockchain (provided by `oasis-net-runner`) and a black-box indexer (i.e. only external APIs are accessed).

The easiest way to do so is via using docker.

## Running e2e tests locally

For the canonical way to run these tests, see the [CI config](../../.github/workflows/ci-test.yaml).
Here are some useful one-liners derived from those commands, and adapted for running locally and repeatedly.

### Get docker containers with current code up&running
```sh
make docker &&
{ docker compose -f tests/e2e/docker-compose.e2e.yml down -t 0 || true; } &&
rm -rf tests/e2e/testnet/net-runner &&
env HOST_UID=$(id -u) HOST_GID=$(id -g) docker compose -f tests/e2e/docker-compose.e2e.yml up -d
```

The `rm` wipes the state for `oasis-net-runner`; state left over from previous runs prevents the runner from starting. (Error: `failed to provision entity: oasis/entity: failed to create deterministic identity: signature/signer/file: key already exists`)

### Run the e2e tests
```sh
docker exec oasis-indexer sh -c "cd /oasis-indexer && make test-e2e"
```

### Check indexer logs
```sh
docker logs oasis-indexer --since 2m -t | less
```
