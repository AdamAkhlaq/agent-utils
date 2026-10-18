# `semver`

Sort, compare, or constraint-check versions per [Semantic Versioning 2.0.0](https://semver.org), with the spec's exact precedence rules (pre-release ordering, numeric vs alphanumeric identifiers, build metadata) instead of a lexical approximation.

## Usage

```sh
agent-utils semver [-r] [file]
agent-utils semver -compare <a> <b>
agent-utils semver -check <constraint> [version]
```

## Flags

| Flag                  | Description                                                                  |
| --------------------- | ---------------------------------------------------------------------------- |
| `-r`                  | Sort in descending order (sort mode only).                                   |
| `-compare`            | Compare two version arguments by precedence: print `-1`, `0`, or `1`.        |
| `-check <constraint>` | Test a version against a constraint: print `true`/`false`, exit `0`/`3`.     |

The three modes are mutually exclusive. By default, versions are read one per line (from `stdin` or a file argument) and sorted ascending by precedence; blank lines are skipped, and any invalid version aborts with its line number and value. Versions with equal precedence (differing only in build metadata or a `v` prefix) are ordered by their original text, byte-wise, so the output is deterministic and independent of input order. `-compare` takes exactly two version arguments; `-check` takes the version as an argument or from `stdin`, and its constraint is space-separated comparators (`=`, `>`, `>=`, `<`, `<=` directly followed by a version), all of which must hold. Ecosystem-specific range syntax (npm's `^`, `~`, `||`) is rejected.

A single leading `v` is accepted on any input version (`v1.2.3`, as in git tags) and preserved in the output. Everything else is strict per spec: partial versions (`1.2`) and leading zeros (`1.02.0`) are rejected with specific errors. Comparison and constraints use pure SemVer precedence, where a pre-release orders below its release: `2.0.0-rc.1` satisfies `<2.0.0`, and `1.2.0-rc.1` does not satisfy `>=1.2.0`.

## Examples

```sh
git tag | agent-utils semver -r | head -1                       # newest tag by precedence
printf '1.0.0-beta.11\n1.0.0-beta.2\n' | agent-utils semver     # beta.2 before beta.11
agent-utils semver -compare 1.2.3 1.10.0                        # -1
agent-utils semver -check ">=1.2.0 <2.0.0" v1.5.3               # true
if agent-utils semver -check ">=1.28.0" "$(kubectl version -o json | jq -r .serverVersion.gitVersion)" >/dev/null; then
  echo "cluster is new enough"
fi
```

---

[All commands](../../README.md#commands)
