The file `/task/fixtures/jobs.json` is a JSON array of scheduled jobs. Write it to `/out/jobs.md` as a GitHub-flavored Markdown table: a header row, the delimiter row, then one row per job in file order.

The columns are every key that appears in any job, in the order the keys first appear: the first job's keys in their written order, then each key that only later jobs have, where it first appears. Write each value as it is written in the file: strings without their quotes, numbers exactly as written (so `0.50` stays `0.50`), and `true` and `false` as they are. A `null` value or a missing key is an empty cell. Escape a pipe inside a cell as `\|`. Pad the cells with spaces so that the pipes line up vertically in every row of the raw Markdown.

Done means `/out/jobs.md` is that table, with its pipes aligned. Do not ask questions; write the file and stop.
