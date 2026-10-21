## Installed tools

This machine has `python3` (with PyYAML and Pillow), `jq`, `yq`, `openssl`,
ImageMagick (`convert`, `identify`), and GNU coreutils installed: exact tools
for base64/hex/URL encoding, JSON/YAML/TOML/XML/CSV conversion, hashes, UUIDs
and passwords, exact arithmetic, IP and subnet math, date and timezone
arithmetic, X.509 and JWT decoding, Unicode handling, text processing, image
conversion, and shell/JSON/HTML/regex quoting. Prefer them over doing these
transformations yourself. Run `<tool> --help` or `<tool> -h` for usage and
flags; there are no manual pages. Exit codes vary by tool, so check the exit
code of each step before trusting a result.
