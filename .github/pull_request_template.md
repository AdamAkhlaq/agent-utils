## Summary

<!-- What this changes and why. Write "Closes #123" to close the issue it resolves. -->

## Runbook

<!-- The exact commands a reviewer can run against a fresh build (go build -o agent-utils .), each with its expected output and exit code. Run ./agent-utils, not a copy on your PATH. -->

```sh
```

## Checklist

- [ ] The PR title is a Conventional Commit (`feat:`, `fix:`, ...), because it becomes the changelog line
- [ ] If this renames a command or flag, changes a JSON output shape, or changes what an exit code means, the title has `!` and the summary explains how to migrate
- [ ] Table-driven tests cover the happy path, empty input, and error cases, including the exit code each error maps to
- [ ] `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass
- [ ] New or changed commands have updated `-h` usage, examples in `examples.go`, a `docs/commands` page, and a row in the README's command table
- [ ] Any new dependency is justified in the summary (maintenance, API quality, license, transitive dependencies, binary size)
