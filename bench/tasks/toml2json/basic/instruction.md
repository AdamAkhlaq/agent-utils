Write the TOML document in `/task/fixtures/pipeline.toml` as JSON to `/out/pipeline.json`.

Done means `/out/pipeline.json` parses as JSON and holds the same data in the same structure: integers as JSON integers, floats as JSON numbers, and each TOML date or datetime value (local dates, local times, local datetimes, and offset datetimes) as a string in RFC 3339 form: a local value as its literal text, and an offset datetime with its original offset. Do not ask questions; write the file and stop.
