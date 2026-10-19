# `commands`

List every command as JSON: the machine-readable counterpart to `agent-utils help`, meant for tools and scripts that build on agent-utils (generating tool schemas for an agent framework, for example).

Each entry carries:

- `name` and a one-line `summary`.
- `usage`: the synopsis lines, one invocation form each.
- `flags`: each with `name` (including the leading `-`), `type` (`boolean`, `integer`, `number`, or `string`, as in JSON Schema), `default` (as a string, empty when the flag has none), and `description`.
- `examples`: each with the shell `command`, plus `output` when the example states exactly what it prints.

`name`, `summary`, `usage`, `flags`, and `examples` are always present, with `[]` when a command has none, so the shape is stable for scripts. The manifest does not yet say which flags are required or mutually exclusive; each command's docs page does. The full manifest is around 20 KB; an agent exploring interactively spends fewer tokens with `agent-utils help` for the overview and `agent-utils <command> -h` for one command.

## Usage

```sh
agent-utils commands
```

## Examples

```sh
agent-utils commands | jq -r '.[].name'                    # command names, one per line
agent-utils commands | jq '.[] | select(.name == "hash")'  # one command's usage, flags, and examples
agent-utils commands | jq -r '.[] | select(.flags == []) | .name'   # commands that take no flags
```

---

[All commands](../../README.md#commands)
