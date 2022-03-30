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

```sh
$ oasis-indexer analyze mainnet 53852332637bacb61b91b6411ab4095168ba02a50be4c3f82448438826f23898 {Unix socket} mainnet ROSE 9
```

## Oasis Node

You should run a local [node](https://docs.oasis.dev/general/run-a-node/set-up-your-node/run-non-validator) for development purposes. You will need the Unix socket.
