include common.mk

# For running `docker compose` containers as current user
export HOST_UID := $(shell id -u)
export HOST_GID := $(shell id -g)

all: build

build:
	@$(ECHO) "$(CYAN)*** Building...$(OFF)"
	@$(MAKE) nexus
	@$(MAKE) docker
	@$(ECHO) "$(CYAN)*** Everything built successfully!$(OFF)"

# Generate Go types from the openapi spec.
# To install the tool, run:
#   go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.12
codegen-go:
	@oapi-codegen --version | grep -qE '^v1.12.' || echo "ERROR: Installed oapi-codegen is not v1.12.x. See Makefile."
	@scripts/namespace_codegen_templates.sh
	@echo $$'compatibility:\n  always-prefix-enum-values: true' > /tmp/codegen-config.yaml
	oapi-codegen -generate types                    -config /tmp/codegen-config.yaml -templates /tmp/namespaced-templates/ -package types api/spec/v1.yaml >api/v1/types/openapi.gen.go
	oapi-codegen -generate chi-server,strict-server -config /tmp/codegen-config.yaml -templates /tmp/namespaced-templates/ -package types api/spec/v1.yaml >api/v1/types/server.gen.go

nexus: codegen-go
	$(GO) build $(GOFLAGS) $(GO_EXTRA_FLAGS)

docker:
	@docker build \
		--tag oasislabs/oasis-node:$(USER)-dev \
		--file docker/oasis-node/Dockerfile \
		docker/oasis-node
	@docker build \
		--tag oasislabs/oasis-indexer:$(USER)-dev \
		--file docker/nexus/Dockerfile \
		.
	@docker build \
		--tag oasislabs/oasis-net-runner:$(USER)-dev \
		--file docker/oasis-net-runner/Dockerfile \
		docker/oasis-net-runner

clean:
	@$(GO) clean

stop-e2e:
	@docker compose -f tests/e2e/docker-compose.e2e.yml down -v -t 0
	rm -rf tests/e2e/testnet/net-runner

# Run tests with parallelism set to 1, since tests in multiple packages can use the same DB.
test:
	@$(GO) test -p 1 -short -v ./...

# Run tests with parallelism set to 1, since tests in multiple packages can use the same DB.
test-ci:
	@$(GO) test -p 1 -race -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic -v ./...

# Run the e2e tests locally, assuming the environment is already set up.
# The recommended usage is via the `start-e2e` target which sets up the environment first.
test-e2e: export OASIS_INDEXER_E2E = true
test-e2e:
	@$(GO) test -race -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic -v ./tests/e2e

fill-cache-for-e2e-regression: nexus
	cp tests/e2e_regression/e2e_config.yml /tmp/nexus_fill_e2e_regression_cache.yml
	sed -i -E 's/query_on_cache_miss: false/query_on_cache_miss: true/g' /tmp/nexus_fill_e2e_regression_cache.yml
	./nexus --config /tmp/nexus_fill_e2e_regression_cache.yml analyze

# Run the api tests locally, assuming the environment is set up with an oasis-node that is
# accessible as specified in the config file.
test-e2e-regression: nexus
	./nexus --config tests/e2e_regression/e2e_config.yml analyze
	@$(ECHO) "$(CYAN)*** Analyzers finished; starting api tests...$(OFF)"
	./tests/e2e_regression/run.sh

# Accept the outputs of the e2e tests as the new expected outputs.
accept-e2e-regression:
	@# Delete all old expected files first, in case any test cases were renamed or removed.
	rm -rf ./tests/e2e_regression/expected
	@# Copy the actual outputs to the expected outputs.
	cp -r  ./tests/e2e_regression/{actual,expected}
	@# The result of the "spec" test is a special case. It should always match the
	@# current openAPI spec file, so we symlink it to avoid having to update the expected
	@# output every time the spec changes.
	rm ./tests/e2e_regression/expected/spec.body
	ln -s  ../../../api/spec/v1.yaml ./tests/e2e_regression/expected/spec.body

# Format code.
fmt:
	@$(ECHO) "$(CYAN)*** Running Go formatters...$(OFF)"
	@gofumpt -w .
	@goimports -w -local github.com/oasisprotocol/nexus .

# Lint code, commits and documentation.
lint-targets := lint-go lint-go-mod-tidy

lint-go: codegen-go
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
	@npx redoc-cli build api/spec/v1.yaml -o api/spec/v1.html

docs: $(docs-targets)

start-docker:
	@docker compose up --remove-orphans

start-docker-e2e:
	@docker compose -f tests/e2e/docker-compose.e2e.yml up -d

start-e2e: start-docker-e2e
	docker exec nexus sh -c "cd /nexus && make test-e2e"

# Run dockerized postgres for local development
postgres:
	@docker ps -a --format '{{.Names}}' | grep -q nexus-postgres && docker start nexus-postgres || \
	docker run \
		--name nexus-postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=rwuser \
		-e POSTGRES_PASSWORD=password \
		-e POSTGRES_DB=indexer \
		-d postgres -c log_statement=all
	@sleep 3  # Experimentally enough for postgres to start accepting connections
	# Create a read-only user to mimic the production environment.
	docker exec nexus-postgres psql -U rwuser indexer -c "CREATE ROLE indexer_readonly; CREATE USER api WITH PASSWORD 'password' IN ROLE indexer_readonly;"

# Attach to the local DB from "make postgres"
psql:
	@docker exec -it nexus-postgres psql -U rwuser indexer

shutdown-postgres:
	@docker rm nexus-postgres --force

release-build: codegen-go
	@goreleaser release --rm-dist

# List of targets that are not actual files.
.PHONY: \
	all build \
	codegen-go \
	nexus \
	test-e2e \
	test-e2e-regression \
	fill-cache-for-e2e-regression \
	accept-e2e-regression \
	docker \
	clean \
	test \
	fmt \
	$(lint-targets) lint \
	$(docs-targets) docs \
	run
