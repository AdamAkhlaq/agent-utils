The file `/task/fixtures/crontab` is the crontab of a server whose clock is set to UTC. For each job in it, find the next three moments the job runs strictly after 2026-11-27T16:50:00Z, following the standard crontab rules from crontab(5). Note that when both the day-of-month and the day-of-week fields are restricted (neither is `*`), a day qualifies if it matches either one of them.

Write a JSON object to `/out/next-runs.json` that maps each job's command (the path at the end of its line) to an array of its next three runs in order, each written as an RFC 3339 timestamp such as `2026-11-27T17:00:00Z`.

Done means `/out/next-runs.json` parses as JSON and holds the next three runs of every job. Do not ask questions; write the file and stop.
