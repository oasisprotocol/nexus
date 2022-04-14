# Oasis Block Indexer

A block indexer for the Oasis Network.

## Docker Environment

You can create and run a Dockerized version of the Oasis Indexer as follows.

**Build**
```sh
$ docker build --tag oasislabs/oasis-indexer:dev --file docker/indexer.Dockerfile .
```

**Run**
```sh
$ alias oasis-indexer="docker run oasislabs/oasis-indexer:dev"
$ oasis-indexer --help
```

**Configuration**

The analyzer service reads a configuration file as the parameters for a network.

```sh
$ oasis-indexer analyze --network.config=config.yaml
```

## Oasis Node

You should run a local [node](https://docs.oasis.dev/general/run-a-node/set-up-your-node/run-non-validator) for development purposes.
You will need the Unix socket for the `config.yaml` file while running an analyzer service.
