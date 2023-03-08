FROM golang:1.19-buster AS oasis-indexer-builder

WORKDIR /code/go

COPY . ./

RUN \
  go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12 && \ 
  make codegen-go && \
  go mod download && \
  go build

############

FROM node:18-slim AS openapi-builder

COPY api/spec /api/spec
WORKDIR /
RUN npx redoc-cli build api/spec/v1.yaml -o api/spec/v1.html

############

FROM golang:1.19-buster AS oasis-indexer

WORKDIR /oasis-indexer

RUN apt-get update -q -q && \
  apt-get install --yes apt-transport-https ca-certificates

COPY --from=oasis-indexer-builder /code/go/oasis-indexer /usr/local/bin/oasis-indexer
COPY --from=oasis-indexer-builder /code/go/storage/migrations /storage/migrations/
COPY --from=oasis-indexer-builder /code/go/ /oasis-indexer
COPY --from=openapi-builder /api/spec/v1.html api/spec/v1.html

ENTRYPOINT ["oasis-indexer"]
