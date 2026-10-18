# `lorem`

Generate lorem ipsum filler text.

## Usage

```sh
agent-utils lorem [-p paragraphs]
agent-utils lorem -w words
```

## Flags

| Flag     | Description                                |
| -------- | ------------------------------------------ |
| `-w <n>` | Number of words.                           |
| `-p <n>` | Number of paragraphs (default 1).          |

`-w` and `-p` are mutually exclusive. Output is deterministic: the canonical Lorem Ipsum passage, cycled for word counts and repeated for paragraphs, so the same command always produces the same text. This command reads no input.

## Examples

```sh
agent-utils lorem                 # one canonical paragraph
agent-utils lorem -p 3            # three paragraphs, blank-line separated
agent-utils lorem -w 40           # exactly 40 words
```

---

[All commands](../../README.md#commands)
