# Contributing to agent-utils

Thanks for helping. Bug reports, command proposals, and pull requests are all welcome.

- **Found a bug?** Open a [bug report](https://github.com/AdamAkhlaq/agent-utils/issues/new?template=bug_report.yml) with the exact command, its input, and what it printed.
- **Want a new command?** Open a [command proposal](https://github.com/AdamAkhlaq/agent-utils/issues/new?template=command_proposal.yml) before writing code, so the name, flags, and output shape can be agreed first. Those are part of the public contract and hard to change later.
- **Have a question?** Ask in [Discussions](https://github.com/AdamAkhlaq/agent-utils/discussions).
- **Found a vulnerability?** Follow the [security policy](SECURITY.md) instead of opening an issue.

## Development setup

You need Go 1.25 or newer.

```sh
git clone https://github.com/AdamAkhlaq/agent-utils.git
cd agent-utils
go build -o agent-utils .
./agent-utils help
```

Before pushing, run the same checks CI runs:

```sh
gofmt -l .          # must print nothing
go vet ./...
go test ./...
```

## How the code is organized

- **`main.go` at the root** builds the command registry and dispatches `os.Args`. A single-binary project doesn't need the `cmd/` layout, which is for repositories that build several binaries.
- **Utility logic lives under `internal/`**, so nothing outside this module can import it. This is an application, not a library.
- **One package per category** (`encode`, `format`, `img`, `inspect`, ...). The package is Go's unit of organization; files within a package are split only for readability. Package names never collide with a standard library package used alongside them (`img`, not `image`).
- **Two layers.** Each utility is a pure, CLI-agnostic core in `internal/<category>`, shaped to its data: `io.Reader` to `io.Writer` for stream transforms, returned values for generators, plain arguments for small inputs. A thin layer in `internal/cli` parses flags and calls it. The `cli` package never imports a core package; `main.go` wires them together.

## Adding a command

Every command must keep [the contract](README.md#the-contract): input from `stdin` or a file argument, results on `stdout`, diagnostics on `stderr`, exit codes `0`/`1`/`2`/`3`, deterministic output, no colors, prompts, or progress output, and network access only when reaching the network is the command's purpose.

1. **Core.** Write the transformation in the right `internal/` package, with table-driven tests (`t.Run` subtests) covering the happy path, empty input, and every error case. Probe any dependency's behavior that matters (ordering, edge cases, silent truncation) with a scratch program first, and encode what you find as tests.
2. **CLI layer.** Reuse an existing builder in `internal/cli` (`EncodeCommand`, `TransformCommand`, `StringTransformCommand`, `ConvertCommand`) before writing a new one. Give the command a one-line `Summary` and a `Usage` synopsis. Return `*cli.UsageError` when the invocation is wrong (an unknown flag, a bad flag value, the wrong number of arguments: exit 2) and `*cli.FalseError` when a yes/no check answers no (exit 3). Any other error exits 1, and that includes malformed data, whether it arrived on `stdin`, in a file, or as a value argument such as a version or a URL. Test that classification as well as the output.
3. **Register it** in `registry()` in `main.go`, and add its examples to `examples.go`. An example's `Output` must be exactly what the command prints; `go test` runs every example that needs no shell features and compares.
4. **Document it** in `docs/commands/<name>.md`, following the existing pages: description, usage, flags table, behavior details, examples. Add a row to the command table in `README.md`. `go test` fails if a command has no page, or if its page leaves out a usage line or a flag.
5. **Verify it end to end.** Build the binary and use the command the way an agent would: pipes, file arguments, bad input, and the exit code of each.

Naming: conversions are `x2y` (`png2jpeg`), everything else is `noun` or `noun-verb` (`uuid`, `json-fmt`). Go functions don't repeat their package name (`generate.UUID`, `img.PNGToJPEG`), and initialisms stay capitalized (`JSON`, `URL`).

**Dependencies:** the standard library comes first. If a third-party module is really needed, explain in the pull request how it does on maintenance, API quality, license, transitive dependencies, and binary size. Code that needs cgo can't be accepted, because it would break the single static binary.

## Pull requests

- Branch from `main` (`feat/<name>`, `fix/<name>`). One command or fix per pull request.
- Pull requests are squash-merged, so the **pull request title** becomes the commit on `main` and a line in the release changelog. Write it as a [Conventional Commit](https://www.conventionalcommits.org): `feat: add the hexdump command`, `fix: reject empty input in json2csv`. Mark breaking changes with `!` (`feat!: ...`).
- Fill in the runbook in the pull request template: the exact commands a reviewer can run, with their expected output and exit codes.
- CI (gofmt, vet, tests, and a build, on both the minimum supported Go version and the latest) must pass before merging.

## Releases

Maintainers release by tagging an approved commit on `main` with `vX.Y.Z` and pushing the tag. The release workflow uses GoReleaser to cross-compile the binaries, generate the changelog from the commit messages, and publish a GitHub Release. Versions follow the [stability policy](README.md#stability): patch releases for fixes, minor releases for new commands, flags, and JSON fields, and major releases for anything that breaks the CLI contract.
