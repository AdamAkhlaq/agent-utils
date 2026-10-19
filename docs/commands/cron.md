# `cron`

Validate a cron expression and list when it will actually run. The parser and scheduler follow [cronie](https://github.com/cronie-crond/cronie) (the cron on Fedora, RHEL, and Arch, descended from Vixie cron like Debian's), including its day-of-month/day-of-week rule and its handling of daylight saving time, and reject anything a standard cron daemon would not run the same way. The expression is a literal argument or comes from `stdin`.

## Usage

```sh
agent-utils cron [-n count] [-z zone] [-from timestamp] [-json] [expression]
```

## Flags

| Flag                | Description                                                                    |
| ------------------- | ------------------------------------------------------------------------------ |
| `-n <count>`        | How many run times to list, 1 to 1000 (default 5).                             |
| `-z <zone>`         | Timezone the schedule runs in: an IANA name, `UTC` (default), or `local`.      |
| `-from <timestamp>` | List runs strictly after this time (default `now`). Accepts any form `time` does; one without an offset is wall-clock time in the `-z` zone. |
| `-json`             | Print the expression, zone, expanded fields, day rule, and run times as JSON.  |

Pass the expression as one quoted argument: unquoted, the shell would split it and expand each `*` into file names, so more than one argument is a usage error. Without an argument, the expression is read from `stdin`, minus one trailing line ending (`\n` or `\r\n`). Flags go before the expression.

Plain output is one RFC 3339 timestamp per line, in the `-z` zone. `agent-utils cron -json -n 2 -from 2026-01-01T00:00:00Z '0 0 1 * MON'` prints:

```json
{
  "expression": "0 0 1 * MON",
  "zone": "UTC",
  "fields": {
    "minute": [0],
    "hour": [0],
    "day_of_month": [1],
    "month": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12],
    "day_of_week": [1]
  },
  "day_match": "either",
  "next": ["2026-01-05T00:00:00Z", "2026-01-12T00:00:00Z"]
}
```

(`fields` is shown compacted here; the real output puts one value per line.) `expression` is the input with its fields separated by single spaces. `fields` lists every value each field allows, with Sunday always `0`. `day_match` is `both` when a day must match the day-of-month and the day-of-week fields, and `either` when matching one is enough (see below).

## Syntax

Five fields separated by spaces or tabs: minute (`0-59`), hour (`0-23`), day of month (`1-31`), month (`1-12` or `JAN`-`DEC`), and day of week (`0-7` or `SUN`-`SAT`, where both `0` and `7` are Sunday). Names are three letters and case-insensitive. Each field is a comma-separated list of:

- `*`, every value;
- a value, `5`, or a range, `1-5` (ranges include both ends, may use names, and never wrap around: write `22-23,0-2`, and `FRI-7` rather than `FRI-SUN`);
- a step on `*` or a range: `*/15` is every 15th value from the first, `1-30/5` is 1, 6, 11, ..., 26.

These macros stand alone in place of the five fields: `@yearly` and `@annually` (`0 0 1 1 *`), `@monthly` (`0 0 1 * *`), `@weekly` (`0 0 * * 0`), `@daily` and `@midnight` (`0 0 * * *`), and `@hourly` (`0 * * * *`). Macros are lowercase, as in cronie.

Each of these fails with a specific error and exit `1`:

- values out of range, unknown names (`Monday`, `January`), empty list elements, and a step of `0`;
- reversed ranges (`17-9`), which current cronie rejects and older cronie and Debian's cron silently treat as selecting nothing;
- a step on a single value (`5/15`): cronie rejects it, and other schedulers read it as `5-59/15`;
- a step that selects only one value: `*/90` in the minute field, which cronie accepts with a warning and runs at minute 0 only, and `*/7` in the day-of-week field, which reaches only Sunday (as `0` and `7`);
- a day-of-week range ending in Sunday as `0` (`FRI-SUN`, `5-0`): cronie reads it as Friday to Sunday, but Debian's and Ubuntu's cron silently selects no weekday for it, so write `FRI-7`;
- 6 or 7 fields (a seconds or year field, as in Quartz and Spring) or a whole crontab line with its command;
- extensions from other schedulers: Quartz's `?`, `L`, `W`, and `#`, Jenkins' `H`, and cronie's random `~`;
- `@reboot`, which runs when the daemon starts and has no schedule;
- a schedule that can never run, such as `0 0 30 2 *` (February 30).

## Day of month and day of week

When both day fields are restricted, a day matches if **either** one matches: `0 0 13 * FRI` runs on every 13th and on every Friday, not only on Friday the 13th. When either field starts with `*`, a day must match **both**, which only matters for a stepped star: `0 0 */2 * FRI` runs on Fridays that fall on an odd day of the month, because cronie decides "restricted" by the field's first character, not by its values. `day_match` in the JSON output says which rule applies.

## Time zones and daylight saving time

The schedule is matched against wall-clock time in the `-z` zone, and around DST changes runs follow cronie's rule for its own time zone, which divides jobs into wildcard jobs (the minute or hour field starts with `*`, or `@hourly`) and fixed-time jobs (everything else):

- **Skipped time** (clocks go forward): a fixed-time job scheduled in the gap runs once when the gap ends; in New York, `30 2 * * *` runs at `03:00-04:00` on the day DST starts. A wildcard job skips the gap.
- **Repeated time** (clocks go back): a fixed-time job runs only the first time; `30 1 * * *` runs at `01:30-04:00`, not again at `01:30-05:00`. A wildcard job runs both times, so `*/20 1 * * *` lists six runs that night, in time order.

Several skipped times that resolve to the same instant are listed once. The same rule is applied to offset changes that are not DST transitions (a zone moving to a new standard offset, as Samoa did when it skipped 2011-12-30); cronie itself only notices an offset change when DST switches on or off, so until it restarts it keeps running on the old offset.

Results must fall within the years `0000` to `9999`, which RFC 3339 can represent. The search covers 400 years past `-from` (or past the last run found), a full cycle of the Gregorian calendar, so rare schedules still resolve: `0 0 29 2 */6` (February 29 on a Saturday or Sunday) runs in 2088 and next in 2116, because 2100 is not a leap year.

A `-from` timestamp without an offset (`2026-03-08` or `2026-03-08 09:00`) is wall-clock time in the `-z` zone, unlike `time`, which reads it as UTC; one with an offset or `Z`, or in epoch seconds, is that exact instant. Output depends on the clock only when `-from` is omitted, and on the machine only with `-z local`.

## Exit codes

`0` on success. `1` when the expression is invalid or never runs, or when fewer than `-n` runs exist before the year 10000. `2` for a usage error: an unknown flag, `-n` outside 1 to 1000, an unknown `-z` zone or unreadable `-from` timestamp (bad flag values), or more than one argument.

## Examples

```sh
agent-utils cron -n 1 -from 2026-01-01T00:00:00Z '30 9 * * MON-FRI'      # 2026-01-01T09:30:00Z
agent-utils cron -n 1 -from 2026-03-01T00:00:00Z '0 0 29 2 *'            # 2028-02-29T00:00:00Z
agent-utils cron -n 1 -z America/New_York -from 2026-03-08 '30 2 * * *'  # 2026-03-08T03:00:00-04:00
agent-utils cron -json '0 0 1 * MON' | jq -r .day_match                  # either
echo '*/15 9-17 * * 1-5' | agent-utils cron -z Europe/London             # next 5 runs, London time
agent-utils cron '0 0 30 2 *'                                            # exit 1: never runs
```

---

[All commands](../../README.md#commands)
