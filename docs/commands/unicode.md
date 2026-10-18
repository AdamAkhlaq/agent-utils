# `unicode`

Inspect text at the codepoint level: what exactly is in a string, character by character. Built for "these strings look identical but don't match" bugs: invisible characters, bidi tricks, and NFC/NFD normalization mismatches.

## Usage

```sh
agent-utils unicode [-check | -nfc | -nfd] [file]
```

## Flags

| Flag     | Description                                                                     |
| -------- | ------------------------------------------------------------------------------- |
| `-check` | Report only suspicious characters and NFC status as JSON, instead of inspecting. |
| `-nfc`   | Write the input normalized to NFC (composed) instead of inspecting.             |
| `-nfd`   | Write the input normalized to NFD (decomposed) instead of inspecting.           |

The three flags are mutually exclusive. The default mode emits a JSON array with one object per codepoint: `char` (the character itself; empty for control `Cc` and format `Cf` codepoints, which are invisible or terminal-mangling, so `codepoint` and `name` identify them), `codepoint` (`U+XXXX`), the official Unicode `name` (empty for unassigned codepoints), the two-letter general `category` (`Lu`, `Zs`, `Cf`, ...; `Cn` for unassigned), and `utf8` (the codepoint's UTF-8 bytes as lowercase hex).

Input is inspected exactly as received: nothing is trimmed first, so a trailing newline shows up as `U+000A`. That is deliberate, because byte-exact diagnosis is the point. `echo` appends a newline; use `printf` to inspect an exact string. Input that is not valid UTF-8 fails loudly with the byte offset of the first invalid byte (exit 1); nothing is ever silently replaced with U+FFFD.

`-check` scans for the characters that cause invisible-difference bugs: zero-width characters (ZWSP, ZWNJ, ZWJ, word joiner), bidi controls, the BOM, non-breaking spaces, and any other format (`Cf`) or control (`Cc`, except tab, LF, and CR) codepoints. Each finding carries its rune `index`, byte `offset`, `codepoint`, `name`, `category`, and a `reason`. The top-level `nfc` field is `false` when the input is not NFC-normalized, a mismatch risk even with no findings. The exit code is 0 whether or not anything is found; the JSON carries the answer, and exit 1 is reserved for real failures like invalid UTF-8. Note that ZWJ is legitimate inside emoji sequences (`👨‍👩‍👧` is joined by them), so a ZWJ finding is not always a bug.

`-nfc` and `-nfd` write the normalized text byte for byte (no trailing newline is added), so the output can be redirected or piped straight back into the inspector.

A worked example, two "identical" strings that don't match because one hides a zero-width space:

```sh
$ printf 'caf\xc3\xa9 \xe2\x80\x8bbar' | agent-utils unicode -check
{
  "nfc": true,
  "findings": [
    {
      "index": 5,
      "offset": 6,
      "codepoint": "U+200B",
      "name": "ZERO WIDTH SPACE",
      "category": "Cf",
      "reason": "zero-width"
    }
  ]
}
```

## Examples

```sh
printf 'e\xcc\x81' | agent-utils unicode               # e + U+0301, two codepoints
printf 'e\xcc\x81' | agent-utils unicode -nfc | agent-utils unicode   # one codepoint: U+00E9
agent-utils unicode -check suspicious.txt | jq .nfc    # false when not NFC-normalized
agent-utils unicode file.txt | jq -r '.[].codepoint'   # codepoints, one per line
```

---

[All commands](../../README.md#commands)
