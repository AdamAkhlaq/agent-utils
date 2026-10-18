## Summary

<!-- What this changes and why. Link the issue it resolves. -->

## Runbook

<!-- The exact commands a reviewer can run against a fresh build (go build -o agent-utils .), each with its expected output and exit code. -->

```sh
```

## Checklist

- [ ] The PR title is a Conventional Commit (`feat:`, `fix:`, ...; `!` for a breaking change), because it becomes the changelog line
- [ ] Table-driven tests cover the happy path, empty input, and error cases, including the exit code each error maps to
- [ ] `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass
- [ ] New or changed commands have updated `-h` usage, examples in `examples.go`, a `docs/commands` page, and a row in the README's command table
- [ ] Any new dependency is justified in the summary (maintenance, license, transitive dependencies, binary size)
