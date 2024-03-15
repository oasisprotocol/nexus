# `git` Hooks

This directory contains suggested git hooks for easier coexistence
with our linters.

To use the hooks, copy them to `<REPO_ROOT>/.git/hooks`.
Hooks include:

- `prepare-commit-msg` - auto-line-wraps the commit message.

Other suggested hooks:

- The hook installed by `gitlint install-hook` - lints each commit message
  before creating the commit.
