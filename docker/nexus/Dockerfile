FROM golang:1.25-trixie AS nexus-builder

WORKDIR /code/go

COPY . ./

RUN \
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 && \
  make codegen-go && \
  go mod download && \
  go build

############

FROM golang:1.25-trixie AS nexus

WORKDIR /nexus

RUN apt-get update -q -q && \
  apt-get install --yes apt-transport-https ca-certificates

COPY --from=nexus-builder /code/go/nexus /usr/local/bin/nexus
COPY --from=nexus-builder /code/go/ /nexus

ENTRYPOINT ["nexus"]
