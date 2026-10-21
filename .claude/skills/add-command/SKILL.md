---
name: add-command
description: Build a new agent-utils command, or a new mode or flag on an existing one, end to end. Covers every file to touch, the design precedents this repo has already set, verification, review, and the PR. Use whenever adding or extending a command.
argument-hint: "<command-name> <what it does>"
---

# Add a command

Target: $ARGUMENTS

CONTRIBUTING.md ("Adding a command") is the checklist of record and CLAUDE.md holds the
contract; read both first. This skill adds what they leave implicit: the decisions to make
before coding, the precedents that answer most of them, and the places a change has to
reach.

## 1. Decide the public surface first

Names, flags, output shapes, and exit codes are public contract (changing them later is a
major release), so settle these before writing code, and surface any judgment call to the
person you're working with:

- **Name**: `x2y` for a conversion, `noun` or `noun-verb` otherwise. A related operation
  on an existing command is a mode or flag there (`time add`), not a new command.
- **Input**: stream (stdin or a file argument), a literal value with a stdin fallback, or
  literal values only. **Output**: plain text for a single value, JSON for a document,
  an output path argument for binary files.
- **Package**: the existing category under `internal/` it belongs to, or a new one-word
  package if none fits (`arith` for `calc`).
- **Limits**: what an adversarial input could make it allocate or loop over, and the cap.
- **Dependency**: standard library first. If one is needed, evaluate it now (see step 5).

## 2. Follow the precedents

Most design questions already have an answer here. Match it unless there's a reason not
to, and say why in the PR when you deviate.

| Question | Precedent |
| --- | --- |
| Literal value or stdin | `color`, `urlparse`, `calc`, `cron`: one literal argument, else stdin. More than one positional argument is a usage error that says to quote the value. |
| Trailing newline on stdin | Text-shaped input strips exactly one (`StringTransformCommand`, `calc`, `cron`, which also strips `\r\n`). Byte-exact commands strip nothing (`base64`, `hash`, `quote`). Document which. |
| Modes | A positional word after the flags: `cidr info`, `time add`, `time diff`. Flags go before the mode; a flag after it is a usage error that says to move it. |
| Error class | Bad data, including a malformed value argument: exit 1. Bad flag value or wrong argument count: exit 2 (`cron -from` garbage is a flag value, so 2). A predicate that answers no: `cli.FalseError`, exit 3. |
| Yes/no output | Print the answer and exit 0 or 3: `true`/`false` (`cidr contains`), nothing (`json-fmt -check`, `hash -c`), or a JSON report (`json-schema`, which always includes `errors`). |
| JSON shape | Pretty-printed, two-space indent, `snake_case` keys, every field always present (empty `[]` rather than omitted) unless it is meaningless for the input (`cidr`'s IPv4-only fields). |
| Big or exact numbers | A JSON string when precision could be lost (`cidr`'s `total_addresses`, `calc`'s `result`); a JSON number when it is exact and `jq` arithmetic matters (`time diff`'s `total_seconds`). |
| Optional numeric flag | `optionalInt` in `internal/cli/flags.go`, so the `commands` manifest doesn't advertise a misleading zero default (`calc -scale`). |
| Time | Inject `time.Now` from `main.go`. Timestamps without an offset go through `clock.ParseIn` in the `-z` zone, which resolves DST gaps forward and repeats to the earlier instant. |
| Output files | Use the existing output helpers in `internal/cli/io.go` (temporary file beside the target, renamed on success, removed on a signal). |
| Bounds | Cap and name the limit in the error, checked before the work starts: `calc` 65536-bit values, `cidr split` 65536 subnets, `cron` a 400-year search, `json-schema` exponents beyond 1000000, image commands 128 megapixels read from the header. |

## 3. Touch every place

- [ ] `internal/<category>/<file>.go` and `_test.go`: the pure core, table-driven tests
  (happy path, empty input, every error), and a fuzz target if it parses anything.
- [ ] `internal/cli/<name>cmd.go` and `_test.go`: flags, argument handling, error
  classification. Reuse a builder (`EncodeCommand`, `TransformCommand`,
  `StringTransformCommand`, `ConvertCommand`) when one fits.
- [ ] `main.go`: register it in `registry()`, alphabetically.
- [ ] `main_test.go`: rows in `TestExitCodes` for success and each error class.
- [ ] `examples.go`: examples with an exact `Output` wherever possible. `TestExamples`
  runs only examples without pipes, redirects, or variables, so check the rest by hand.
- [ ] `docs/commands/<name>.md`: description, usage, flags table, details (including
  limits and exit codes), examples. Follow an existing page of the same shape.
- [ ] `README.md`:
  - the command table row;
  - the agent snippet under "Use it with your agent" (a capability phrase, wrapped at
    80 columns without splitting a code span);
  - "The contract": the list of literal-value commands, the exit-3 checks, and the
    determinism exceptions, whichever the command joins.
- [ ] `SECURITY.md` "Scope", only if the command reaches the network, runs a program, or
  writes somewhere other than stdout or a named output path.
- [ ] `bench/tasks/<name>/<case>/`: at least one benchmark task (`task.toml`,
  `instruction.md`, fixtures, `oracle.sh`, `check.sh`), written to the rules in
  `bench/README.md`: the instruction states a goal and an output path under `/out/` and
  names no tool; the checker accepts any correct answer. `go run ./bench validate -tasks
  '<name>/*'` must pass, Docker step included. A command without a task fails the
  coverage test once the suite is complete.

## 4. Verify

```sh
gofmt -l .                  # prints nothing
go mod tidy -diff
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
go test -count=1 ./...
go test -race ./...
go test -run '^$' -fuzz '^FuzzX$' -fuzztime 60s ./internal/<category>  # for each new fuzz target
```

Then build the binary into a temporary directory and drive it the way an agent would:
literal argument, stdin, file argument, empty input, malformed input, a bad flag, `-h`,
`agent-utils commands | jq '.[] | select(.name == "<name>")'`, and the exit code of every
case. Try the inputs from step 1's limits question and confirm they fail fast.

## 5. Dependencies

Before adding a module, probe it with a scratch program outside the repo (ordering, edge
cases, silent truncation, network access, resource use) and turn the findings into tests.
The PR states maintenance, API quality, license, transitive dependencies, the binary size
delta (`go build -ldflags='-s -w'` before and after), and the startup cost: package
`init` work runs on every command, not just this one.

## 6. Review

- `/code-review` on the diff, for correctness.
- The `contract-reviewer` agent, for the agent-first contract and docs drift.
- The `hardening-auditor` agent when the command parses untrusted formats, allocates in
  proportion to input, or adds a dependency.

Fix what holds up, and record what you declined and why.

## 7. Pull request

Branch `feat/<name>` (or `fix/<name>`), title as a Conventional Commit (`feat: add the
<name> command ...`). Fill in `.github/pull_request_template.md`: summary, design
decisions (including any deviation from the precedents above), dependency evaluation if
any, a runbook of exact commands against `./agent-utils` with expected output and exit
codes, and the checklist. Commit, push, and merge only when the person you're working
with asks.

When several command branches are in flight, they all touch `main.go`, `main_test.go`,
`examples.go`, and the same README lines. Rebase each onto the updated `main` before
merging, re-wrap the agent snippet, then rebuild and rerun the full suite, because two
branches changing one package can break the build without a textual conflict.
