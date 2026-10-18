# `case`

Convert identifier casing: snake, camel, pascal, kebab, or SCREAMING_SNAKE.

## Usage

```sh
agent-utils case -to <snake|camel|pascal|kebab|screaming> [file]
```

## Flags

| Flag           | Description                                                          |
| -------------- | -------------------------------------------------------------------- |
| `-to <target>` | Target casing: `snake`, `camel`, `pascal`, `kebab`, or `screaming`. Required. |

Words are detected by splitting on runs of non-alphanumeric characters (spaces, `-`, `_`, punctuation) and at camel boundaries: before an upper letter that follows a lower letter or digit, and before the last upper of an all-upper run followed by a lower letter, so acronym runs stay whole (`HTTPServer` is `HTTP` + `Server`). Every word is then lowercased before joining, so acronyms normalize (`HTTPServer` in camel is `httpServer`, not `hTTPServer`); preserving their capitalization would need a dictionary.

## Examples

```sh
printf "camelCase" | agent-utils case -to snake       # camel_case
printf "snake_case" | agent-utils case -to camel      # snakeCase
printf "kebab-case" | agent-utils case -to pascal     # KebabCase
printf "HTTPServer" | agent-utils case -to snake      # http_server
printf "hello world" | agent-utils case -to screaming # HELLO_WORLD
agent-utils case -to kebab name.txt
```

---

[All commands](../../README.md#commands)
