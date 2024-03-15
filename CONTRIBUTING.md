# Contributing Guidelines

Thank you for your interest in contributing to Oasis Nexus! There are many ways
to contribute, and this document should not be considered encompassing.

If you have a general question on how to use and deploy our software, please
read our [General Documentation](https://docs.oasis.dev) or join our
[community Discord](https://oasis.io/discord).

For concrete feature requests and/or bug reports, please file an issue in this
repository as described below.

<!-- markdownlint-disable heading-increment -->

#### Table of Contents

<!-- markdownlint-enable heading-increment -->

[Feature Requests](#feature-requests)

[Bug Reports](#bug-reports)

[Development](#development)

- [Building](#building-and-testing)
- [Contributing Code](#contributing-code)
- [Style Guides](#style-guides)
  - [Git Commit Messages](#git-commit-messages)
  - [Go Style Guide](#go-style-guide)

## Feature Requests

To request new functionality the most appropriate place to propose it is as a
[new Feature request] in this repository.

<!-- markdownlint-disable line-length -->

[new feature request]:
  https://github.com/oasisprotocol/nexus/issues/new?template=feature_request.md

<!-- markdownlint-enable line-length -->

## Bug Reports

Bugs are a reality for any software project. We can't fix what we don't know
about!

If you believe a bug report presents a security risk, please follow
[responsible disclosure](https://en.wikipedia.org/wiki/Responsible_disclosure)
and report it directly to <security@oasisprotocol.org> instead of filing a
public issue or posting it to a public forum. We will get back to you promptly.

Otherwise, please, first search between [existing issues in our repository] and
if the issue is not reported yet, [file a new one].

<!-- markdownlint-disable line-length -->

[existing issues in our repository]:
  https://github.com/oasisprotocol/nexus/issues
[file a new one]:
  https://github.com/oasisprotocol/nexus/issues/new?template=bug_report.md

<!-- markdownlint-enable line-length -->

## Development

### Building and Testing

Building and testing are documented in our
[README](https://github.com/oasisprotocol/nexus/blob/main/README.md).

### Contributing Code

- **File issues:** Please make sure to first file an issue (i.e. feature
  request, bug report) before you actually start work on something.

- **Create branches:** If you have write permissions to the repository, you can
  create user-id prefixed branches (e.g. user/feature/foobar) in the main
  repository. Otherwise, fork the main repository and create your branches
  there.

  - Good habit: regularly rebase to the `HEAD` of `main` branch of the main
    repository to make sure you prevent nasty conflicts:

    ```bash
    git rebase <main-repo>/main
    ```

  - Push your branch to GitHub regularly so others can see what you are working
    on:

    ```bash
    git push -u <main-repo-or-your-fork> <branch-name>
    ```

    _Note that you are allowed to force push into your development branches._

- _main_ branch is protected and will require at least 1 code review approval
  from a code owner before it can be merged.

- When coding, please follow these standard practices:

  - **Write tests:** Especially when fixing bugs, make a test so we know that
    we’ve fixed the bug and prevent it from reappearing in the future.
  - **Logging:** Please follow the logging conventions in the rest of the code
    base.
  - **Instrumentation:** Please follow the instrumentation conventions in the
    rest of the code.
    - Try to instrument anything that would be relevant to an operational
      network.

- **Change Log:** This project generates release changelogs automatically using
  commit messages. Please follow commit format as described in the
  [Style guide](#git-commit-messages).

- **Check CI:** Don’t break the build!

  - Make sure all tests pass before submitting your pull request for review.

- **Signal PR review:**

  - Mark the draft pull request as _Ready for review_.
  - Please include good high-level descriptions of what the pull request does.
  - The description should include references to all GitHub issues addressed by
    the pull request. Include the status ("done", "partially done", etc).
  - Provide some details on how the code was tested.
  - After you are nearing review (and definitely before merge) **squash commits
    into logical parts** (avoid squashing over merged commits, use rebase
    first!). Use proper commit messages which explain what was changed in the
    given commit and why.

- **Get a code review:**

  - Code owners will be automatically assigned to review based on the files that
    were changed.
  - You can generally look up the last few people to edit the file to get the
    best person to review.
  - When addressing the review: Make sure to address all comments, and respond
    to them so that the reviewer knows what has happened (e.g. "done" or
    "acknowledged" or "I don't think so because ...").

- **Merge:** Once approved, the creator of the pull request should merge the
  branch, close the pull request, and delete the branch. If the creator does not
  have write access to the repository, one of the committers should do so
  instead.

- **Signal to close issues:** Let the person who filed the issue close it. Ping
  them in a comment (e.g. @user) making sure you’ve commented how an issue was
  addressed.
  - Anyone else with write permissions should be able to close the issue if not
    addressed within a week.

### Style Guides

Style consistency is largely enforced by linters in CI. This section reviews
some of the rules and gives tips on how to auto-fix most linting errors.

#### Git Commit Messages

A quick summary:

- Separate subject from body with a blank line.
- Keep the subject line and body lines short (see gitlint config).
- Do not end the subject line with a period.
- Use the body to explain _what_ and _why_ changed, more so than _how_. This
  applies especially to PR descriptions, but also large commits.

For potentially useful git hooks that lint (and autoformat!) locally, see
[scripts/git-hooks](scripts/git-hooks/README.md).

#### Go Style Guide

Go code should use the [`gofumpt`](https://github.com/mvdan/gofumpt) formatting
style. Be sure to run `make fmt` before pushing any code.

It is a good idea to set up "Format on save", available in most editors.

#### Markdown

Configuration to auto-format markdown in VSCode as much as possible:

- In your personal settings:

  ```json
  "editor.codeActionsOnSave": {
    "source.fixAll.markdownlint": "always"
  },
  "[markdown]": {
    "editor.formatOnSave": true,
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  },
  ```

  The former will auto-fix lots of issues around consistent indentation, inline
  markers, etc. The latter will auto-wrap the lines. However, you do need to
  also configure Prettier. Install the `esbenp.prettier-vscode` extension, then
  in its settings, set

  - `prettier.proseWrap` to `always` (default: `preserve`)
  - `prettier.printWidth` to 80 (or whatever `.markdownlint.yml` prescribes)
