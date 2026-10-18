# `slugify`

Turn text into a lowercase hyphenated slug: runs of anything that isn't a letter or digit become single hyphens, with none leading or trailing.

## Usage

```sh
agent-utils slugify [file]
```

Unicode letters are kept and lowercased, not transliterated (`Café` becomes `café`, not `cafe`).

## Examples

```sh
printf "Hello, World!" | agent-utils slugify   # hello-world
printf "A -- Messy___Title (2024)" | agent-utils slugify   # a-messy-title-2024
agent-utils slugify title.txt
```

---

[All commands](../../README.md#commands)
