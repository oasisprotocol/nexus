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
#   go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
codegen-go:
	@oapi-codegen --version | grep -qE '^v2.4.1' || echo "ERROR: Installed oapi-codegen is not v2.4.1. See Makefile."
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

E2E_REGRESSION_SUITES_NO_LINKS := eden_testnet_2025 eden_2025 eden damask
# To run specific suites, do e.g.
# make E2E_REGRESSION_SUITES='suite1 suite2' test-e2e-regression
E2E_REGRESSION_SUITES := $(E2E_REGRESSION_SUITES_NO_LINKS) edenfast

E2E_REGRESSION_ARTIFACTS_VERSION = 2025-05-30

upload-e2e-regression-caches:
	for suite in $(E2E_REGRESSION_SUITES); do \
		cache_dir="tests/e2e_regression/$$suite/rpc-cache"; \
		archive_path="/tmp/$$suite-rpc-cache.tzst"; \
		echo "Compressing $$cache_dir to $$archive_path..."; \
		tar -cf "$$archive_path" --exclude='*.backup' -C "$$(dirname $$cache_dir)" "$$(basename $$cache_dir)" \
			--use-compress-program zstd; \
		echo "Uploading $$archive_path to S3..."; \
		aws s3 cp "$$archive_path" \
			s3://oasis-artifacts/nexus-tests-e2e-regression/$(E2E_REGRESSION_ARTIFACTS_VERSION)/$$suite/rpc-cache.tzst; \
	done

ensure-e2e-regression-caches:
	for suite in $(E2E_REGRESSION_SUITES); do \
		cache_path="tests/e2e_regression/$$suite/rpc-cache"; \
		archive_url="https://oasis-artifacts.s3.amazonaws.com/nexus-tests-e2e-regression/$(E2E_REGRESSION_ARTIFACTS_VERSION)/$$suite/rpc-cache.tzst"; \
		archive_tmp="/tmp/$$suite-rpc-cache.tzst"; \
		if [ "$$FORCE" = "1" ]; then \
			echo "Forcing download of $$cache_path..."; \
			rm -rf "$$cache_path"; \
		fi; \
		if [ ! -d "$$cache_path" ]; then \
			echo "Downloading $$archive_url..."; \
			curl -L -o "$$archive_tmp" "$$archive_url"; \
			echo "Extracting to $$cache_path..."; \
			mkdir -p "$$(dirname $$cache_path)"; \
			tar -xf "$$archive_tmp" -C "$$(dirname $$cache_path)" --use-compress-program unzstd; \
		else \
			echo "Cache already exists for $$suite — skipping."; \
		fi; \
	done

ensure-consistent-config-for-e2e-regression:
	for suite in $(E2E_REGRESSION_SUITES); do ./tests/e2e_regression/ensure_consistent_config.sh $$suite; done

# Run the api tests locally, assuming the environment is set up with an oasis-node that is
# accessible as specified in the config file.
test-e2e-regression: nexus ensure-consistent-config-for-e2e-regression ensure-e2e-regression-caches
	for suite in $(E2E_REGRESSION_SUITES); do ./tests/e2e_regression/run.sh -a $$suite; done

# Accept the outputs of the e2e tests as the new expected outputs.
accept-e2e-regression:
	for suite in $(E2E_REGRESSION_SUITES_NO_LINKS); do ./tests/e2e_regression/accept.sh $$suite; done

# Format code.
fmt:
	@$(ECHO) "$(CYAN)*** Running Go formatters...$(OFF)"
	@gofumpt -w .
	@goimports -w -local github.com/oasisprotocol/nexus .

# Lint code, commits and documentation.
lint-targets := lint-go lint-go-mod-tidy lint-changelog lint-git lint-docs

lint-go: codegen-go
	@$(ECHO) "$(CYAN)*** Running Go linters...$(OFF)"
	@env -u GOPATH golangci-lint run

lint-git:
	@$(CHECK_GITLINT)

lint-docs:
	@$(ECHO) "$(CYAN)*** Running markdownlint-cli...$(OFF)"
	@npx markdownlint-cli '**/*.md' --ignore .changelog/

lint-changelog:
	@$(CHECK_CHANGELOG_FRAGMENTS)

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
	@docker compose -f tests/e2e/docker-compose.e2e.yml up -d --build

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

# Fetch all the latest changes (including tags) from the canonical upstream git
# repository.
fetch-git:
	@$(ECHO) "Fetching the latest changes (including tags) from $(GIT_ORIGIN_REMOTE) remote..."
	@git fetch $(GIT_ORIGIN_REMOTE) --tags

# Private target for bumping project's version using the Punch tool.
# NOTE: It should not be invoked directly.
_version-bump: fetch-git
	@$(ENSURE_VALID_RELEASE_BRANCH_NAME)
	@$(PUNCH_BUMP_VERSION)
	@git add $(PUNCH_VERSION_FILE)

# Private target for assembling the Change Log.
# NOTE: It should not be invoked directly.
_changelog:
	@$(ECHO) "$(CYAN)*** Generating Change Log for version $(PUNCH_VERSION)...$(OFF)"
	@$(BUILD_CHANGELOG)
	@$(ECHO) "Next, review the staged changes, commit them and make a pull request."
	@$(WARN_BREAKING_CHANGES)

# Assemble Change Log.
# NOTE: We need to call Make recursively since _version-bump target updates
# Punch's version and hence we need Make to re-evaluate the PUNCH_VERSION
# variable.
changelog: _version-bump
	@$(MAKE) --no-print-directory _changelog

# Tag the next release.
release-tag: fetch-git
	@$(ECHO) "Checking if we can tag version $(PUNCH_VERSION) as the next release..."
	@$(ENSURE_VALID_RELEASE_BRANCH_NAME)
	@$(ENSURE_RELEASE_TAG_DOES_NOT_EXIST)
	@$(ENSURE_NO_CHANGELOG_FRAGMENTS)
	@$(ENSURE_NEXT_RELEASE_IN_CHANGELOG)
	@$(ECHO) "All checks have passed. Proceeding with tagging the $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH)'s HEAD with tag '$(RELEASE_TAG)'."
	@$(CONFIRM_ACTION)
	@$(ECHO) "If this appears to be stuck, you might need to touch your security key for GPG sign operation."
	@git tag --sign --message="Version $(PUNCH_VERSION)" $(RELEASE_TAG) $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH)
	@git push $(GIT_ORIGIN_REMOTE) $(RELEASE_TAG)
	@$(ECHO) "$(CYAN)*** Tag '$(RELEASE_TAG)' has been successfully pushed to $(GIT_ORIGIN_REMOTE) remote.$(OFF)"

release-build: codegen-go
	@goreleaser $(GORELEASER_ARGS)

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
	_version-bump _changelog changelog \
	fetch-git \
	release-tag \
	$(lint-targets) lint \
	$(docs-targets) docs \
	run
