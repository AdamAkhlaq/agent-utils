# `uuid`

Generate random (version 4) UUIDs, one per line.

## Usage

```sh
agent-utils uuid [-n count]
```

## Flags

| Flag         | Description                          |
| ------------ | ------------------------------------ |
| `-n <count>` | How many to generate (default 1).    |

UUIDs are drawn from the operating system's cryptographically secure random source. This command reads no input.

## Examples

```sh
agent-utils uuid          # e.g. 8b28f3f4-9d51-4a7b-b8a2-52c62c54cbf5
agent-utils uuid -n 5     # five, one per line
```

---

[All commands](../../README.md#commands)
