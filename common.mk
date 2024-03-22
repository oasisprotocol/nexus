# Use Bash shell.
# NOTE: You can control which Bash version is used by setting the PATH
# appropriately.
SHELL := bash

# Path to the directory of this Makefile.
# NOTE: Prepend all relative paths in this Makefile with this variable to ensure
# they are properly resolved when this Makefile is included from Makefiles in
# other directories.
SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

# Function for comparing two strings for equality.
# NOTE: This will also return false if both strings are empty.
eq = $(and $(findstring $(1),$(2)), $(findstring $(2),$(1)))

# Check if we're running in an interactive terminal.
ISATTY := $(shell [ -t 0 ] && echo 1)

# If running interactively, use terminal colors.
ifdef ISATTY
	MAGENTA := \e[35;1m
	CYAN := \e[36;1m
	RED := \e[0;31m
	OFF := \e[0m
	ECHO_CMD := echo -e
else
	MAGENTA := ""
	CYAN := ""
	RED := ""
	OFF := ""
	ECHO_CMD := echo
endif

# Output messages to stderr instead stdout.
ECHO := $(ECHO_CMD) 1>&2

# Boolean indicating whether to assume the 'yes' answer when confirming actions.
ASSUME_YES ?= 0

# Helper that asks the user to confirm the action.
define CONFIRM_ACTION =
	if [[ $(ASSUME_YES) != 1 ]]; then \
		$(ECHO) -n "Are you sure? [y/N] " && read ans && [ $${ans:-N} = y ]; \
	fi
endef

# Version of the markdownlint-cli package to use.
MARKDOWNLINT_CLI_VERSION := 0.26.0

# Name of git remote pointing to the canonical upstream git repository, i.e.
# git@github.com:oasisprotocol/<repo-name>.git.
GIT_ORIGIN_REMOTE ?= origin

# Name of the branch where to tag the next release.
RELEASE_BRANCH ?= main

# Determine project's version from git.
# NOTE: This computes the project's version from the latest version tag
# reachable from current git commit and does not search for version
# tags across the whole $(GIT_ORIGIN_REMOTE) repository.
GIT_VERSION := $(subst v,,$(shell \
	git describe --tags --match 'v*' --abbrev=0 2>/dev/null HEAD || \
	echo undefined \
))

# Determine project's version.
# If the current git commit is exactly a tag and it equals the Punch version,
# then the project's version is that.
# Else, the project version is the Punch version appended with git commit and
# dirty state info.
GIT_COMMIT_EXACT_TAG := $(shell \
	git describe --tags --match 'v*' --exact-match &>/dev/null HEAD && echo YES || echo NO \
)

# Go binary to use for all Go commands.
export OASIS_GO ?= go

# Go command prefix to use in all Go commands.
GO := env -u GOPATH $(OASIS_GO)

# NOTE: The -trimpath flag strips all host dependent filesystem paths from
# binaries which is required for deterministic builds.
GOFLAGS ?= -trimpath -v

VERSION := $(or \
	$(and $(call eq,$(GIT_COMMIT_EXACT_TAG),YES), $(GIT_VERSION)), \
	$(shell git describe --tags --abbrev=0)-git$(shell git describe --always --match '' --dirty=+dirty 2>/dev/null) \
)

# Project's version as the linker's string value definition.
export GOLDFLAGS_VERSION := -X github.com/oasisprotocol/nexus/version.Software=$(VERSION)

# Go's linker flags.
export GOLDFLAGS ?= "$(GOLDFLAGS_VERSION)"

# Helper that ensures the git workspace is clean.
define ENSURE_GIT_CLEAN =
	if [[ ! -z `git status --porcelain` ]]; then \
		$(ECHO) "$(RED)Error: Git workspace is dirty.$(OFF)"; \
		exit 1; \
	fi
endef

# Helper that checks if the go mod tidy command was run.
# NOTE: go mod tidy doesn't implement a check mode yet.
# For more details, see: https://github.com/golang/go/issues/27005.
define CHECK_GO_MOD_TIDY =
    $(GO) mod tidy; \
    if [[ ! -z `git status --porcelain go.mod go.sum` ]]; then \
        $(ECHO) "$(RED)Error: The following changes detected after running 'go mod tidy':$(OFF)"; \
        git diff go.mod go.sum; \
        exit 1; \
    fi
endef

# Helper that checks commits with gitlilnt.
# NOTE: gitlint internally uses git rev-list, where A..B is asymmetric
# difference, which is kind of the opposite of how git diff interprets
# A..B vs A...B.
define CHECK_GITLINT =
	BRANCH=$(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH); \
	COMMIT_SHA=`git rev-parse $$BRANCH` && \
	$(ECHO) "$(CYAN)*** Running gitlint for commits from $$BRANCH ($${COMMIT_SHA:0:7})... $(OFF)"; \
	gitlint --commits $$BRANCH..HEAD
endef

# List of non-trivial Change Log fragments.
CHANGELOG_FRAGMENTS_NON_TRIVIAL := $(filter-out $(wildcard .changelog/*trivial*.md),$(wildcard .changelog/[0-9]*.md))

# List of breaking Change Log fragments.
CHANGELOG_FRAGMENTS_BREAKING := $(wildcard .changelog/*breaking*.md)

# Helper that checks Change Log fragments with markdownlint-cli.
# NOTE: Non-zero exit status is recorded but only set at the end so that all
# markdownlint errors can be seen at once.
define CHECK_CHANGELOG_FRAGMENTS =
	exit_status=0; \
	$(ECHO) "$(CYAN)*** Running markdownlint-cli for Change Log fragments... $(OFF)"; \
	[[ "$$(markdownlint --version || true)" == $(MARKDOWNLINT_CLI_VERSION) ]] && \
		mdlint_bin=markdownlint || \
		{ mdlint_bin="npx markdownlint-cli@$(MARKDOWNLINT_CLI_VERSION)"; $(ECHO) "$(RED)Note: Install markdownlint locally with "'`npm install -g markdownlint-cli@$(MARKDOWNLINT_CLI_VERSION)`'" to speed up linting.$(OFF)"; }; \
	$$mdlint_bin --config .changelog/.markdownlint.yml .changelog/ || exit_status=$$?; \
	$(ECHO) "$(CYAN)*** Running gitlint for Change Log fragments: $(OFF)"; \
	for fragment in $(CHANGELOG_FRAGMENTS_NON_TRIVIAL); do \
		$(ECHO) "- $$fragment"; \
		true TODO: USE GITLINT WHEN AVAILABLE IN CI gitlint --msg-filename $$fragment -c title-max-length.line-length=78 --staged || exit_status=$$?; \
	done; \
	exit $$exit_status
endef

# Helper that builds the Change Log.
define BUILD_CHANGELOG =
	if [[ $(ASSUME_YES) != 1 ]]; then \
		towncrier build --version $(RELEASE_VERSION); \
	else \
		towncrier build --version $(RELEASE_VERSION) --yes; \
	fi
endef

# Helper that prints a warning when breaking changes are indicated by Change Log
# fragments.
define WARN_BREAKING_CHANGES =
	if [[ -n "$(CHANGELOG_FRAGMENTS_BREAKING)" ]]; then \
		$(ECHO) "$(RED)Warning: This release contains breaking changes.$(OFF)"; \
		$(ECHO) "$(RED)         Make sure the version is bumped appropriately.$(OFF)"; \
	fi
endef

# Helper that ensures the origin's release branch's HEAD doesn't contain any
# Change Log fragments.
define ENSURE_NO_CHANGELOG_FRAGMENTS =
	if ! CHANGELOG_FILES=`git ls-tree -r --name-only $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH) .changelog`; then \
		$(ECHO) "$(RED)Error: Could not obtain Change Log fragments for $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH) branch.$(OFF)"; \
		exit 1; \
	fi; \
	if CHANGELOG_FRAGMENTS=`echo "$$CHANGELOG_FILES" | grep --invert-match --extended-regexp '(README.md|template.md.j2|.markdownlint.yml)'`; then \
		$(ECHO) "$(RED)Error: Found the following Change Log fragments on $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH) branch:"; \
		$(ECHO) "$${CHANGELOG_FRAGMENTS}$(OFF)"; \
		exit 1; \
	fi
endef

# Helper that ensures the origin's release branch's HEAD contains a Change Log
# section for the next release.
define ENSURE_NEXT_RELEASE_IN_CHANGELOG =
	if ! ( git show $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH):CHANGELOG.md | \
			grep --quiet '^## $(RELEASE_VERSION) (.*)' ); then \
		$(ECHO) "$(RED)Error: Could not locate Change Log section for release $(RELEASE_VERSION) on $(GIT_ORIGIN_REMOTE)/$(RELEASE_BRANCH) branch.$(OFF)"; \
		exit 1; \
	fi
endef

# Helper that ensures the new release's tag doesn't already exist on the origin
# remote.
define ENSURE_RELEASE_TAG_DOES_NOT_EXIST =
	if git ls-remote --exit-code --tags $(GIT_ORIGIN_REMOTE) $(RELEASE_TAG) 1>/dev/null; then \
		$(ECHO) "$(RED)Error: Tag '$(RELEASE_TAG)' already exists on $(GIT_ORIGIN_REMOTE) remote.$(OFF)"; \
		exit 1; \
	fi; \
	if git show-ref --quiet --tags $(RELEASE_TAG); then \
		$(ECHO) "$(RED)Error: Tag '$(RELEASE_TAG)' already exists locally.$(OFF)"; \
		exit 1; \
	fi
endef

# Helper that ensures $(RELEASE_BRANCH) variable contains a valid release branch
# name.
define ENSURE_VALID_RELEASE_BRANCH_NAME =
	if [[ ! $(RELEASE_BRANCH) =~ ^(main|(stable/[0-9]+\.[0-9]+\.x$$)) ]]; then \
		$(ECHO) "$(RED)Error: Invalid release branch name: '$(RELEASE_BRANCH)'."; \
		exit 1; \
	fi
endef

define newline


endef

# GitHub release text in Markdown format.
define RELEASE_TEXT =
For a list of changes in this release, see the [Change Log].

*NOTE: If you are upgrading from an earlier release, please **carefully review**
the [Change Log] for **Removals and Breaking changes**.*

[Change Log]: https://github.com/oasisprotocol/nexus/blob/v$(VERSION)/CHANGELOG.md

endef

GORELEASER_ARGS ?= release --rm-dist
# If the appropriate environment variable is set, create a real release.
ifeq ($(NEXUS_REAL_RELEASE), true)
# Create temporary file with GitHub release's text.
_RELEASE_NOTES_FILE := $(shell mktemp /tmp/nexus.XXXXX)
_ := $(shell printf "$(subst ",\",$(subst $(newline),\n,$(RELEASE_TEXT)))" > $(_RELEASE_NOTES_FILE))
GORELEASER_ARGS = release --release-notes $(_RELEASE_NOTES_FILE)
endif
