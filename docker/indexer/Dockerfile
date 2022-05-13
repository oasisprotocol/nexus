FROM golang:1.17-buster AS oasis-indexer-builder

WORKDIR /code/go

COPY . ./

RUN go mod download && \
    go build -o indexer oasis-indexer/main.go

FROM golang:1.17-buster AS oasis-indexer

COPY --from=oasis-indexer-builder /code/go/indexer /usr/local/bin/oasis-indexer

ENTRYPOINT ["oasis-indexer"]
