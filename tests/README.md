# Oasis Indexer Tests

This directory contains, primarily, e2e tests for the Oasis Indexer. They have been organized as follows:

- `genesis/` contains tests that validate our database state is correct against `oasis-node` genesis state.
- `http/` contains tests that validate API endpoints behave as expected and return correct output.

## Setup for E2E Tests

To ensure that tests behave as expected, you should have either the local environment or Docker environment configured as per the [top-level README](../README.md#docker-development).

Then you can run these tests from the repository root with `make test-ci` or directly from this directory with `go test ./... -v`.
