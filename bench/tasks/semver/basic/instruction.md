The file `/task/fixtures/tags.txt` lists a project's release tags as `git tag` prints them, in byte order. Each tag is a `v` followed by a Semantic Versioning 2.0.0 version, some with a pre-release part or build metadata. Write the tags to `/out/sorted.txt`, one per line, from the lowest to the highest precedence as the Semantic Versioning 2.0.0 specification defines it, each written exactly as it appears in the file. No two tags have equal precedence.

Done means `/out/sorted.txt` lists every tag once, in that order. Do not ask questions; write the file and stop.
