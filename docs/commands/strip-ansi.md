# `strip-ansi`

Remove ANSI escape sequences from text: the classic cleanup for CI logs and captured terminal output before parsing them. Strips CSI sequences (colors including 256-color and truecolor, cursor movement, erase-line as used by progress bars, private `?`-prefixed modes), OSC strings terminated by BEL or ST (window titles, OSC 8 hyperlinks), the other ECMA-48 strings (DCS, SOS, PM, APC), and short ESC sequences such as `ESC ( B` and `ESC =`.

## Usage

```sh
agent-utils strip-ansi [input [output]]
```

Only escape sequences are removed: `\r`, `\n`, `\t`, and all plain text, including multi-byte UTF-8, pass through untouched, so input with no escapes comes out byte-identical. The input is streamed in constant memory, so arbitrarily large logs are fine. A sequence truncated at EOF is dropped silently rather than emitted half-stripped. Reads stdin or an input-file positional; an optional second positional names an output file.

## Examples

```sh
printf '\033[31mred\033[0m plain\n' | agent-utils strip-ansi   # red plain
agent-utils strip-ansi ci-log.txt
agent-utils strip-ansi raw.log clean.log
npm test 2>&1 | agent-utils strip-ansi | grep FAIL
```

---

[All commands](../../README.md#commands)
