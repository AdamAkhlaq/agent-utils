# `hash`

Compute a checksum of data, or verify one.

## Usage

```sh
agent-utils hash [-a algo] [file]
agent-utils hash [-a algo] -c <hex> [file]
```

## Flags

| Flag        | Description                                                                           |
| ----------- | ------------------------------------------------------------------------------------- |
| `-a <algo>` | Hash algorithm: `sha256` (default), `sha1`, `sha512`, or `md5`.                       |
| `-c <hex>`  | Verify: compare the digest to this checksum (case-insensitive, whitespace trimmed).   |

Input is streamed through the hasher, so hashing large files is fine. `md5` and `sha1` are provided for integrity checks and content addressing, not for security. Verify mode prints nothing on success and communicates through the exit code (`0` match, `3` mismatch, `1` only for a real failure such as an unreadable file), making it script- and agent-friendly.

## Examples

```sh
printf "abc" | agent-utils hash                        # ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
agent-utils hash release.tar.gz                        # sha256 of the file
agent-utils hash -a md5 release.tar.gz                 # md5 instead
agent-utils hash -c "$(cut -d' ' -f1 release.sha256)" release.tar.gz && echo "ok"
```

---

[All commands](../../README.md#commands)
