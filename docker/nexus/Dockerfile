FROM golang:1.24-bookworm AS nexus-builder

WORKDIR /code/go

COPY . ./

RUN \
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 && \
  make codegen-go && \
  go mod download && \
  go build

############

FROM node:18-slim AS openapi-builder

COPY api/spec /api/spec
WORKDIR /
RUN npx redoc-cli build api/spec/v1.yaml -o api/spec/v1.html

############

FROM golang:1.24-bookworm AS nexus

WORKDIR /nexus

RUN apt-get update -q -q && \
  apt-get install --yes apt-transport-https ca-certificates

COPY --from=nexus-builder /code/go/nexus /usr/local/bin/nexus
COPY --from=nexus-builder /code/go/storage/migrations /storage/migrations/
COPY --from=nexus-builder /code/go/ /nexus
COPY --from=openapi-builder /api/spec/v1.html api/spec/v1.html

ENTRYPOINT ["nexus"]
