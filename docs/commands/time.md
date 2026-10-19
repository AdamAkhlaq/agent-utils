# `time`

Print or convert a timestamp, add a duration to one, or find the difference between two. With no argument, prints the current time; with one, parses it from any accepted form: `now`, epoch seconds or milliseconds (told apart by magnitude), RFC 3339, RFC 1123, or `2006-01-02` with an optional `15:04[:05]`. The `add` and `diff` modes take timestamps in the same forms, with one difference: there, a timestamp without an offset (`2024-01-31`, `2024-01-31 09:00`) is a wall-clock time in the `-z` zone, because that is the calendar they compute in. The plain form keeps reading it as UTC and converting it to `-z`, as it always has. Epoch values, `now`, and timestamps with an offset are the same instant everywhere. Timestamps and durations are always arguments: `time` never reads `stdin`, so `echo 1755459000 | agent-utils time` prints the current time. Pass piped values with `agent-utils time "$(cat ts.txt)"`.

## Usage

```sh
agent-utils time [-z zone] [-f format | -layout layout | -json] [timestamp]
agent-utils time [-z zone] [-f format | -layout layout | -json] add <timestamp> <duration>
agent-utils time [-z zone] [-json] diff <from> <to>
```

## Modes

| Mode | Description |
| ---- | ----------- |
| `[timestamp]` | Print the timestamp (default: now) in the chosen output form. |
| `add <timestamp> <duration>` | Print the timestamp moved by the duration, in the chosen output form. |
| `diff <from> <to>` | Print `to` minus `from` as an ISO 8601 duration, or every form of it with `-json`. |

A malformed timestamp or duration, or an `add` result out of range, is bad data and exits `1`. The wrong number of arguments, an unknown mode, a bad flag or flag value, and a flag placed after the mode are usage errors and exit `2`.

## Flags

| Flag           | Description                                                        |
| -------------- | ------------------------------------------------------------------ |
| `-z <zone>`    | Timezone for output and for calendar arithmetic: an IANA name, `UTC` (default), or `local`. |
| `-f <format>`  | Output format: `rfc3339` (default), `unix`, `unix-ms`, `date`, `time`. Not valid with `diff`. |
| `-layout <l>`  | Custom Go time layout instead of `-f`. Not valid with `diff`.      |
| `-json`        | Print every representation at once as JSON.                        |

Output defaults to RFC 3339 in UTC, deliberately: local-zone defaults would make output machine-dependent. `-f`, `-layout`, and `-json` are mutually exclusive. The IANA timezone database is embedded in the binary, so zones work identically on every platform, including Windows.

Flags go before the mode word (or before the timestamp). Everything after `add` or `diff` is taken literally, so negative durations and epochs (`-90s`, `-P1D`, `-1`) need no `--`, though a `--` right after the mode is accepted. A flag placed after the mode or the timestamp is a usage error that says to move it.

## JSON output

`-json` on a timestamp (or on the result of `add`) prints `unix`, `unix_ms`, `rfc3339`, `utc`, `date`, `time`, `weekday`, `iso_year`, `iso_week`, and `zone`. The zone-dependent fields use the `-z` zone. `iso_year` and `iso_week` are the ISO 8601 week date, which can differ from the calendar year near New Year: 2021-01-03 is in week 53 of 2020.

## Adding a duration

`add` prints the timestamp moved by the duration, in the same output forms as a plain timestamp. A duration is either:

- **ISO 8601**: `P1Y2M10DT2H30M`, `P2W`, `PT1.5S`, with a leading `-` to go back in time. Only seconds may have a fraction (up to nanoseconds); write 1.5 hours as `PT1H30M`.
- **Go-style**: `1h30m`, `-90s`, `250ms`, with units `h`, `m`, `s`, `ms`, `us`, and `ns`. A fraction finer than a nanosecond (`1.5ns`) is an error rather than being dropped. There is deliberately no `d` unit, because a day is not always 24 hours: `3d` is an error that suggests `P3D` (calendar days) or `72h` (exact hours). Go-style durations are limited to about 292 years.

The parts apply in a fixed order:

1. **Years and months together**, on the wall clock of the `-z` zone. A day past the end of the target month clamps to its last day, as in most date libraries: `2024-01-31` plus `P1M` is `2024-02-29`, and `2024-02-29` plus `P1Y` is `2025-02-28`. (Go's `AddDate` would roll over into March instead.)
2. **Weeks and days**, also on the wall clock, so the time of day stays the same across a DST change: in `America/New_York`, `P1D` from noon on 2024-03-09 is noon on 2024-03-10, 23 real hours later.
3. **The time part** (`T...`) and Go-style durations, as exact elapsed time: `PT24H` from that same noon is 13:00 the next day.

When a calendar step lands on a wall-clock time that a DST change skips, the result moves forward by the gap (02:30 on a spring-forward day becomes 03:30); when it lands on a time that occurs twice, the result is the earlier one. A zone-less timestamp given to `add` or `diff` resolves the same way. The calendar parts use the `-z` zone even when the input timestamp carries a different offset, so pass `-z` for the zone whose days and months you mean. A result outside the range of 64-bit epoch milliseconds (about 292 million years either side of 1970) is an error. RFC 3339 output (`-f rfc3339` and the `rfc3339` and `utc` JSON fields) shows whole seconds; to see a sub-second result, use `-f unix-ms` or `-layout 2006-01-02T15:04:05.999999999Z07:00`.

## Differences

`diff` computes `to` minus `from` and prints it as an ISO 8601 duration: the most whole months that fit, then the most whole days, then the exact remainder in hours, minutes, and seconds, all on the wall clock of the `-z` zone. It is the duration that `add` turns `from` back into `to`, so `agent-utils time add <from> "$(agent-utils time diff <from> <to>)"` always prints the `to` instant. A negative difference has a leading `-` (`-P1M1D`), and an empty one is `PT0S`. Because months vary in length, and because a wall-clock time that a DST change repeats resolves to its earlier instant, `diff a b` is not always the negation of `diff b a` near the end of a month or across a fall-back change.

With `-json`, `diff` prints:

| Field           | Meaning |
| --------------- | ------- |
| `from`, `to`    | The two timestamps, RFC 3339 in the `-z` zone, with fractional seconds when they have any. |
| `zone`          | The zone used for the calendar parts. |
| `duration`      | The ISO 8601 duration that plain `diff` prints. |
| `total_seconds` | The exact elapsed time in seconds, as a JSON number with a fraction only when an input has sub-second precision. |
| `total_days`    | The most whole calendar days that fit, as `P<n>D` would count them. In UTC this is the elapsed time divided by 86400, rounded toward zero. |
| `calendar`      | The parts of `duration` as numbers: `years`, `months`, `days`, `hours`, `minutes`, and `seconds` (which may have a fraction). |

Every number carries the sign of the difference. The arithmetic is exact over the whole range of accepted timestamps; nothing saturates or rounds.

## Examples

```sh
agent-utils time                              # e.g. 2026-08-17T19:45:20Z
agent-utils time 1755459000                   # epoch seconds to RFC 3339
agent-utils time -z Asia/Tokyo 1755459000     # same instant, Tokyo wall clock
agent-utils time -f unix "2026-08-17T19:45:20Z"   # RFC 3339 to epoch
agent-utils time -json now | jq -r .weekday   # e.g. Monday
agent-utils time add 2024-01-31 P1M           # 2024-02-29T00:00:00Z
agent-utils time -f date add now P45D         # the date 45 days from today
agent-utils time -z America/New_York add 2024-03-09T12:00:00-05:00 P1D   # 2024-03-10T12:00:00-04:00
agent-utils time -z America/New_York -json diff 2024-03-01 2024-04-01 | jq .total_days   # 31
agent-utils time add now -90s                 # 90 seconds ago
agent-utils time diff 2024-01-31 2024-03-01T12:00:00Z   # P1M1DT12H
agent-utils time diff 2024-03-01 2024-01-31   # -P1M1D
agent-utils time -json diff 2026-01-01 2026-12-25 | jq .total_days   # 358
```

---

[All commands](../../README.md#commands)
