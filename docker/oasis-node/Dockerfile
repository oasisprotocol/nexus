FROM golang:1.24-bookworm AS oasis-node

ARG OASIS_CORE_VERSION=25.3

WORKDIR /code/go

# Fetch and extract oasis-node binary.
RUN wget https://github.com/oasisprotocol/oasis-core/releases/download/v${OASIS_CORE_VERSION}/oasis_core_${OASIS_CORE_VERSION}_linux_amd64.tar.gz && \
    tar -xf oasis_core_${OASIS_CORE_VERSION}_linux_amd64.tar.gz && \
    mv /code/go/oasis_core_${OASIS_CORE_VERSION}_linux_amd64/oasis-node /usr/local/bin/ && \
    rm -rf /code/go/oasis_core_${OASIS_CORE_VERSION}_linux_amd64*

# Create working directory and user for running Oasis Non-validator Node
# https://docs.oasis.dev/general/run-a-node/set-up-your-node/run-non-validator/
RUN mkdir -m700 -p /node/{etc,data} && \
    adduser --system oasis --shell /usr/sbin/nologin

USER oasis
