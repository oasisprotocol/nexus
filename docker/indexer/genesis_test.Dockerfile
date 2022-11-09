# This image is currently pushed to the oasislabs/oasis-indexer:genesis_test tag

FROM golang:1.18-buster AS oasis-indexer-builder

WORKDIR /code/go

COPY . ./

RUN go mod download && \
    go build

FROM golang:1.18-buster AS oasis-indexer

WORKDIR /oasis-indexer

RUN apt-get update -q -q && \
  apt-get install --yes apt-transport-https ca-certificates

COPY --from=oasis-indexer-builder /code/go/oasis-indexer /usr/local/bin/oasis-indexer
COPY --from=oasis-indexer-builder /code/go/storage/migrations /storage/migrations/
COPY . /oasis-indexer

ENV OASIS_INDEXER_HEALTHCHECK=1

CMD ["oasis-indexer"]
