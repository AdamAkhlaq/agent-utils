# `time`

Print or convert a timestamp. With no argument, prints the current time; with one, parses it from any accepted form: `now`, epoch seconds or milliseconds (told apart by magnitude), RFC 3339, RFC 1123, or `2006-01-02` with an optional `15:04[:05]`. The timestamp is always an argument: `time` never reads `stdin`, so `echo 1755459000 | agent-utils time` prints the current time. Pass piped values with `agent-utils time "$(cat ts.txt)"`.

## Usage

```sh
agent-utils time [-z zone] [-f format | -layout layout | -json] [timestamp]
```

## Flags

| Flag           | Description                                                        |
| -------------- | ------------------------------------------------------------------ |
| `-z <zone>`    | Output timezone: an IANA name, `UTC` (default), or `local`.        |
| `-f <format>`  | Output format: `rfc3339` (default), `unix`, `unix-ms`, `date`, `time`. |
| `-layout <l>`  | Custom Go time layout instead of `-f`.                             |
| `-json`        | Print every representation at once as JSON.                        |

Output defaults to RFC 3339 in UTC, deliberately: local-zone defaults would make output machine-dependent. `-f`, `-layout`, and `-json` are mutually exclusive. The IANA timezone database is embedded in the binary, so zones work identically on every platform, including Windows.

## Examples

```sh
agent-utils time                              # e.g. 2026-08-17T19:45:20Z
agent-utils time 1755459000                   # epoch seconds to RFC 3339
agent-utils time -z Asia/Tokyo 1755459000     # same instant, Tokyo wall clock
agent-utils time -f unix "2026-08-17T19:45:20Z"   # RFC 3339 to epoch
agent-utils time -json now                    # unix, unix_ms, rfc3339, utc, date, time, weekday, zone
```

---

[All commands](../../README.md#commands)
