# Docker Development

This folder contains Docker images required for running Oasis Nexus locally:

- `oasis-node`
- `oasis-indexer` (AKA Nexus)

## Prebuilt Images

Oasis Labs provides prebuilt
[`oasis-indexer`](https://hub.docker.com/repository/docker/oasislabs/oasis-indexer)
and `oasis-node` images that are compatible with the provided development
environment. You can select a release of your choosing, or `latest` and specify
the appropriate tag in the root `docker-compose.yml` to develop with these
versions.

## Build Image Locally

To build the `oasis-indexer` and `oasis-node` images for Docker development, go
to the `nexus` repository root and run:

```sh
make docker
```

This will build both images with the `dev` tag, and will enable you to run

```sh
make start-docker
```

to start the development environment with [`docker
compose`]((<https://docs.docker.com/compose/>).
