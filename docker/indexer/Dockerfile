FROM golang:1.17-buster AS oasis-indexer-builder

WORKDIR /code/go

COPY . ./

RUN go mod download && \
    go build

FROM golang:1.17-buster AS oasis-indexer

COPY --from=oasis-indexer-builder /code/go/oasis-indexer /usr/local/bin/oasis-indexer
COPY --from=oasis-indexer-builder /code/go/storage/migrations /storage/migrations/

ENTRYPOINT ["oasis-indexer"]
