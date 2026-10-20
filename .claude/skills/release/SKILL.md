---
name: release
description: Cut an agent-utils release. Picks the SemVer bump from the commits since the last tag, previews the changelog, tags only after explicit approval, then verifies the published release the way a user installs it.
disable-model-invocation: true
argument-hint: "[version, e.g. v2.1.0 or v2.1.0-rc.1]"
---

# Release

Requested version: $ARGUMENTS (empty means: propose one)

A release is a tag on `main`. Pushing it runs `.github/workflows/release.yml`, which uses
GoReleaser (`.goreleaser.yaml`) to cross-compile, build the changelog from commit
subjects, and publish the GitHub Release. A pushed tag is public and permanent in
practice, so nothing in step 4 happens without an explicit go-ahead in this
conversation.

## 1. Preconditions

```sh
git fetch --tags origin
git status --short                                     # nothing that matters uncommitted
gh run list --branch main --workflow ci --limit 1      # latest main run: completed, success
gh pr list --state open                                # anything meant for this release still open?
```

Stop and report if `main`'s CI isn't green, or if an open PR looks like it was meant to be
in this release.

## 2. Decide the version

```sh
last=$(git tag --list 'v*' --sort=-v:refname | grep -v -- - | head -1)
git log --format='%h %s' "$last"..origin/main
```

- Any subject with `!` before the colon (`feat!:`), or a change that renames a command or
  flag, changes a JSON output shape, or changes an exit code's meaning: **major**.
- Otherwise any `feat`: **minor**.
- Otherwise any `fix` or `perf`: **patch**.
- Only `docs`, `test`, `chore`, `ci`, `build`, or `refactor`: there is nothing for users;
  say so and stop unless asked to release anyway.

Read the diffs of anything ambiguous rather than trusting the subject: a `fix:` that
changes output shape is breaking. If a version was requested, check it against this and
point out any mismatch. A pre-release (`-rc.N`) is published as a GitHub pre-release and
never becomes `releases/latest`.

## 3. Preview and ask

Show the person:

- the proposed tag and the commit it points at (`git rev-parse --short origin/main`);
- the changelog as GoReleaser will group it: Breaking changes, Features, Fixes, Other
  (`build`, `ci`, `refactor`, `perf`), with `docs`, `chore`, and `test` left out;
- anything users should know that the subjects don't say (new limits, behavior changes);
- whether README.md describes anything not in this release.

Then wait. Only an explicit approval of this exact tag counts.

## 4. Tag and publish

Tags here are lightweight and point at a commit on `main`:

```sh
git tag vX.Y.Z "$(git rev-parse origin/main)"
git push origin vX.Y.Z
gh run watch "$(gh run list --workflow release --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
```

If the workflow fails, report the failing step and stop. Never delete, move, or re-push a
tag, and never edit or delete a release, without being asked.

## 5. Verify what users get

```sh
gh release view vX.Y.Z --json assets -q '.assets[].name'
```

Expect six archives (`agent-utils_{linux,darwin,windows}_{amd64,arm64}`, `.zip` for
Windows, `.tar.gz` otherwise) plus `checksums.txt`. Then, in an empty temporary
directory, install it exactly as the README's quick start does, and check it:

```sh
base=https://github.com/AdamAkhlaq/agent-utils/releases/download/vX.Y.Z
asset="agent-utils_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz"
curl -fsSLO "$base/$asset" -O "$base/checksums.txt"
shasum -a 256 --check --ignore-missing checksums.txt   # <asset>: OK (sha256sum on Linux)
tar -xzf "$asset" agent-utils
./agent-utils version                                  # vX.Y.Z
./agent-utils commands | jq length                     # the number of registered commands
```

For a full release, also confirm that `releases/latest` now resolves to it and that
`go install github.com/adamakhlaq/agent-utils/v2@vX.Y.Z` succeeds (the module proxy can
take a few minutes).

Report the release URL, the version, and the verification results.
