FROM golang:1.17-buster AS oasis-indexer-builder

WORKDIR /code/go

COPY go/go.mod go/go.sum go/ go/config/test/network.yaml ./

RUN go mod download && \
    go build -o oasis-block-indexer oasis-indexer/main.go

FROM golang:1.17-buster AS oasis-indexer

COPY --from=oasis-indexer-builder /code/go/oasis-block-indexer /usr/local/bin/
COPY docker/indexer/network.yaml /config/network.yaml

ENTRYPOINT ["oasis-block-indexer"]
