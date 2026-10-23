The file `/task/fixtures/pipeline.yaml` is a CI configuration that uses YAML anchors, aliases, and merge keys (`<<`). Write the fully resolved document as JSON to `/out/pipeline.json`, keeping every top-level key: each alias replaced by the value it refers to, and each merge key applied as YAML defines it, so that no `<<` key remains.

Done means `/out/pipeline.json` parses as JSON and holds the same data a YAML loader with merge-key support produces. Do not ask questions; write the file and stop.
