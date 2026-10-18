# `password`

Generate random passwords, one per line, from a cryptographically secure random source.

## Usage

```sh
agent-utils password [-l length] [-n count] [-no-symbols]
```

## Flags

| Flag          | Description                                       |
| ------------- | ------------------------------------------------- |
| `-l <len>`    | Password length (default 20).                     |
| `-n <count>`  | How many to generate (default 1).                 |
| `-no-symbols` | Letters and digits only.                          |

Characters are drawn uniformly (no modulo bias) from letters, digits, and the symbols `!@#$%^&*-_=+?`, a set chosen to paste safely into shells and config files. The default 20 characters with symbols gives roughly 124 bits of entropy. This command reads no input.

## Examples

```sh
agent-utils password
agent-utils password -l 32 -n 5
agent-utils password -no-symbols
```

---

[All commands](../../README.md#commands)
