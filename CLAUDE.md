# CLAUDE.md

Guidance for AI coding agents working in this repository. People contributing by hand
should start with [CONTRIBUTING.md](CONTRIBUTING.md), which holds the full checklist;
this file adds the rules and context an agent needs to apply it well.

`agent-utils` is an **agent-first CLI**: a single static Go binary of small, exact
utilities (encode, format, convert, generate, inspect) built to be called by AI agents,
with shell scripts and humans as equally welcome users. Every change ships complete:
logic, tests, docs, and an end-to-end check of the built binary.

## Mission

An agent should reach for `agent-utils` instead of doing a transformation "by hand" in
the model, because the tool is instant, token-free, and exact. Every design decision
starts from the question: what makes this trivially usable by an agent?

## Agent-first contract (every command must satisfy this)

- **Deterministic**: same input, same output. Randomness only where randomness is the
  point (`uuid`, `password`), and then from `crypto/rand`. The clock is injected from
  `main.go`, never read inside a core package.
- **Predictable I/O**: read `stdin` or an optional file argument, write results to
  `stdout`, diagnostics to `stderr`. Never prompt, never color, never animate.
- **Exit codes carry meaning**: 0 success, 1 runtime failure (including bad data in a
  value argument), 2 usage error, 3 a yes/no check answered no (return
  `cli.FalseError`). An agent must be able to branch on the code alone.
- **Structured output for structured data**: if the natural result is a document, emit
  pretty-printed JSON with `snake_case` keys that pipes into `jq`.
- **Loud failures**: bad input and misused flags produce a specific, actionable error
  and a non-zero exit; nothing is silently ignored, defaulted, or truncated.
- **Bounded resources**: input is untrusted. A small input must never cost unbounded
  memory or time (decompression bombs, alias expansion, huge exponents, deep nesting,
  searches that never end); cap the work and fail with an error that names the limit.
- **Discoverable**: the command registers a one-line summary (surfaced by both the help
  text and the `commands` JSON manifest) and documents its flags via `-h`.
- **Offline by default**: network access only where the utility's purpose is network
  access (`video`), and stated plainly in its docs.

## How to work

1. **Plan briefly first**: what you're about to build, how it fits the structure below,
   and any decision worth surfacing (naming, flags, output shape, a dependency). Command
   names, flags, and output shapes are public contract, so agree them before building.
2. **Implement completely**: two-layer design, table-driven tests, wrapped errors,
   `gofmt`/`go vet` clean. No placeholder code; everything written should still belong
   here when the tool has 100 commands. For a new command or mode, use the
   `/add-command` skill: it lists every file to touch and the design precedents
   already set.
3. **Verify end to end**: build the binary and exercise the change the way an agent
   would (pipes, files, error cases, exit codes) before calling it done. For a bug,
   reproduce it with the built binary first, so the fix addresses the real failure.
4. **Review before the PR**: run the `contract-reviewer` agent on any change to a
   command's behavior, and the `hardening-auditor` agent on anything that parses
   untrusted formats, allocates in proportion to input, or adds a dependency.
5. **Update the docs**: `docs/commands/<name>.md`, examples in `examples.go`, and the
   README (command table, the agent snippet, and the lists in "The contract"). `go test`
   enforces the page and example basics. Docs describe only what the binary does.
6. **Be candid**: call out what's weak or debatable in your own code; flag trade-offs
   instead of glossing over them.

## Code standards

- **Two layers, always**: a pure, CLI-agnostic core in `internal/<category>`; a thin CLI
  layer in `internal/cli` parses flags and calls it, and `main.go` wires the two (the
  `cli` package never imports a core package). Core signatures match the data shape:
  `io.Reader` -> `io.Writer` for stream transforms, returned values for generators,
  plain arguments for small inputs. Reuse the existing CLI builders (`EncodeCommand`,
  `TransformCommand`, `StringTransformCommand`, `ConvertCommand`) before writing a new
  one.
- **Tests ship with the code**, table-driven with `t.Run` subtests, covering happy path,
  empty input, and error cases, including the exit code each error maps to
  (`TestExitCodes` in `main_test.go`). Parsers get a fuzz target that is kept.
- **Idiomatic over clever; standard library first.** Any third-party dependency is
  justified in the PR (maintenance, API quality, license, transitive dependencies,
  binary size, and startup cost) before it is added. No cgo.
- **Probe dependencies empirically** before trusting them: verify the behaviors that
  matter (ordering, edge cases, silent truncation, resource use) with a scratch
  program, and encode the findings as tests.
- **No over-engineering.** Build for the utility in front of us; no CLI framework, and
  the hand-rolled command registry stays until it's a real bottleneck.
- **Comments explain why**, not what: a non-obvious decision, a workaround, a
  constraint the code can't express. Readable code gets no comment.

## Naming

- **Packages:** lowercase, one word, no underscores; never colliding with a stdlib
  package we'd import alongside (`img`, not `image`). One package per category under
  `internal/`, split into files only for readability.
- **Functions:** no stutter with the package prefix: `generate.UUID`, `img.PNGToJPEG`
  (`XToY` for conversions), `encode.Base64`. Initialisms stay capitalized (`JSON`,
  `URL`, `QR`, `JWT`).
- **Commands:** `x2y` for conversions (`png2jpeg`); `noun`/`noun-verb` otherwise
  (`uuid`, `json-fmt`, `jwt-decode`).

## Writing style

Docs, help text, and PR descriptions are plain, precise American English. Never use the
em dash; restructure with a colon, comma, or parentheses instead. Hyphens belong only in
flags and compound words.

## Workflow: PRs and releases

- **All changes land through pull requests.** `main` is protected: branch
  (`feat/<name>`, `fix/<name>`), commit, push, and open a PR with `gh pr create`
  following the template, including a runbook (exact commands, expected output, and exit
  codes) so the change can be verified independently. Merge only after CI is green, by
  squash, so `main` stays one commit per change. Never push to `main` directly, and
  commit or push only when the person you're working with asks.
- **Conventional Commits** (`feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`,
  `ci`, `chore`): the PR title becomes the commit on `main` and the changelog line, and
  CI checks it. Breaking changes use `!` (`feat!:`) and get their own changelog section.
- **Stacked or parallel PRs**: rebase each onto the updated `main` before merging, then
  rebuild and run the full suite. A rebase without textual conflicts can still break the
  build when two branches change the same package.
- **Releases are tags**, cut by a maintainer with the `/release` skill. SemVer: patch for
  fixes, minor for new commands, flags, or JSON fields, major for breaking the CLI
  contract (renamed commands or flags, changed output shapes, exit-code meaning). Never
  create or push a tag unless a maintainer asks for that release.

## Planning

There is no roadmap file. Future utilities and larger ideas are tracked as GitHub
issues; each utility ships complete in its own PR.

## Don't

- Ship code without tests, or without exercising the built binary end to end.
- Add a dependency without the evaluation in the PR.
- Break the agent-first contract: no interactivity, no color, no silent behavior, no
  unbounded work.
- Over-engineer or add abstractions ahead of need.
- Present planned behavior as current in README.md.
