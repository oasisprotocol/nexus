FROM golang:1.18-buster AS oasis-core

WORKDIR /code/go

# Fetch and extract oasis-node binary.
RUN wget https://github.com/oasisprotocol/oasis-core/releases/download/v22.1.10/oasis_core_22.1.10_linux_amd64.tar.gz && \
    tar -xf oasis_core_22.1.10_linux_amd64.tar.gz && \
    mv /code/go/oasis_core_22.1.10_linux_amd64/oasis-node /usr/local/bin/ && \
    mv /code/go/oasis_core_22.1.10_linux_amd64/oasis-net-runner /usr/local/bin/ && \
    rm -rf /code/go/oasis_core_22.1.10_linux_amd64*

RUN mkdir -m700 -p /testnet
