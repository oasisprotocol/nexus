# Oasis Nexus Tests

This directory contains tests for Oasis Nexus. They have been organized as
follows:

- `e2e/` contains tests that run against a fake blockchain (provided by
  `oasis-net-runner`) and a black-box Nexus (i.e. only external APIs are
  accessed).
- `genesis/` contains tests that validate our database state is correct against
  `oasis-node` genesis state.
- `http/` contains tests that validate API endpoints behave as expected and
  return correct output.

## E2E Tests

To ensure that tests behave as expected on CI, you should have Docker
environment configured as per the
[top-level README](../README.md#docker-development).

### Running e2e tests locally

For the canonical way to run these tests, see the
[CI config](../../.github/workflows/ci-test.yaml). Here are some useful
one-liners derived from those commands, and adapted for running locally and
repeatedly.

#### Build docker containers with current code

```sh
make docker
```

#### Run the e2e tests

This line will clean up any prior runs and start a new run of tests inside the
`nexus` container:

```sh
make stop-e2e && make start-e2e
```

`make stop-e2e` wipes the state for `oasis-net-runner`; state left over from
previous runs prevents the runner from starting.
(Error: `failed to provision entity: oasis/entity:
failed to create deterministic identity: signature/signer/file: key already exists`)

#### Check Nexus logs

```sh
docker logs nexus --since 2m -t | less
```

This is the dockerized nexus against which the e2e tests ran.
