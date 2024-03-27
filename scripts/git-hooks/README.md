# `git` Hooks

This directory contains suggested git hooks for easier coexistence with our
linters.

To use the hooks, copy or link them to `<REPO_ROOT>/.git/hooks`. To link all of
them:

```bash
for f in scripts/git-hooks/*; do
  link=".git/hooks/$(basename "$f")"
  target="../../scripts/git-hooks/$f"
  echo "Linking $link to point to $target"
  ln -s "$target" "$link"
done
```

If you already have hooks in place, you'll have to merge the old and new hooks
manually. In particular, in this repo, you're likely using `git-lfs`, which
creates its own `pre-push` hook. One option is to call the new hook from the old
one:

```bash
echo $'\n\nscripts/git-hooks/pre-push' >>".git/hooks/pre-push"
```

Hooks include:

- `prepare-commit-msg` - auto-line-wraps the commit message.
- `pre-commit` - autoformats the changelog fragments.
- `pre-push` - warns if the PR is missing a changelog fragment.

Other suggested hooks:

- The hook installed by `gitlint install-hook` - lints each commit message
  before creating the commit.
