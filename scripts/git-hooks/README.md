# `git` Hooks

This directory contains suggested git hooks for easier coexistence
with our linters.

To use the hooks, copy or link them to `<REPO_ROOT>/.git/hooks`. To link all of
them:

```bash
for f in scripts/git-hooks/*; do
  ln -s ../../"$f" ".git/hooks/$(basename "$f")"
done
```

(If you already have hooks in place, you'll have to merge the old and new hooks
manually.)

Hooks include:

- `prepare-commit-msg` - auto-line-wraps the commit message.
- `pre-commit` - autoformats the changelog fragments.

Other suggested hooks:

- The hook installed by `gitlint install-hook` - lints each commit message
  before creating the commit.
