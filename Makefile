include common.mk

all: build

build:
	@$(ECHO) "$(CYAN)*** Building...$(OFF)"
	@$(MAKE) oasis-indexer
	@$(MAKE) docker
	@$(ECHO) "$(CYAN)*** Everything built successfully!$(OFF)"

oasis-indexer:
	@$(GO) build $(GOFLAGS) $(GO_EXTRA_FLAGS)

docker:
	@docker build \
		--tag oasislabs/oasis-node:dev \
		--file docker/oasis-node/Dockerfile \
		docker/oasis-node
	@docker build \
		--tag oasislabs/oasis-indexer:dev \
		--file docker/indexer/Dockerfile \
		.
	@docker build \
		--tag oasislabs/oasis-net-runner:dev \
		--file docker/oasis-net-runner/Dockerfile \
		docker/oasis-net-runner

clean:
	@$(GO) clean

test:
	@$(GO) test -short -v ./...

test-ci:
	@$(GO) test -race -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic -v ./...

test-e2e:
	@$(GO) test -race -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic -v ./tests/e2e

# Format code.
fmt:
	@$(ECHO) "$(CYAN)*** Running Go formatters...$(OFF)"
	@gofumpt -w .
	@goimports -w -local github.com/oasislabs/oasis-indexer .

# Lint code, commits and documentation.
lint-targets := lint-go lint-go-mod-tidy

lint-go:
	@$(ECHO) "$(CYAN)*** Running Go linters...$(OFF)"
	@env -u GOPATH golangci-lint run

lint-go-mod-tidy:
	@$(ECHO) "$(CYAN)*** Checking go mod tidy...$(OFF)"
	@$(ENSURE_GIT_CLEAN)
	@$(CHECK_GO_MOD_TIDY)

lint: $(lint-targets)

# Documentation
docs-targets := docs-api

docs-api:
	@npx redoc-cli build api/spec/v1.yaml -o index.html

docs: $(docs-targets)

start-docker:
	@docker compose up --remove-orphans	

# Run dockerized postgres for local development
postgres:
	@docker ps --format '{{.Names}}' | grep -q indexer-postgres && docker start indexer-postgres || \
	docker run \
		--name indexer-postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=rwuser \
		-e POSTGRES_PASSWORD=password \
		-e POSTGRES_DB=indexer \
		-d postgres

# Attach to the local DB from "make postgres"
psql:
	@docker exec -it indexer-postgres psql -U rwuser indexer

shutdown-postgres:
	@docker rm postgres --force

release-build:
	@goreleaser release --rm-dist

# List of targets that are not actual files.
.PHONY: \
	all build \
	oasis-indexer \
	docker \
	clean \
	test \
	fmt \
	$(lint-targets) lint \
	$(docs-targets) docs \
	run