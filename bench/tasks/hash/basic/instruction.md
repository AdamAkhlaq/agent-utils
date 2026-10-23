A release directory holds three files: `/task/fixtures/release/install.sh`, `/task/fixtures/release/manifest.json`, and `/task/fixtures/release/notes.md`. Write a SHA-256 checksum list for them to `/out/SHA256SUMS`: one line per file, sorted by file name, each line being the file's SHA-256 digest as 64 lowercase hexadecimal digits, two spaces, and the bare file name (such as `notes.md`). The digests cover each file's exact bytes.

Done means `/out/SHA256SUMS` holds exactly those three lines. Do not ask questions; write the file and stop.
