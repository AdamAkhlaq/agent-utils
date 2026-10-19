# `quote`

Quote or escape text as an exact literal for a target syntax: a shell word, a JSON string, HTML text, or a regular expression. With `-d`, decode a literal back to the raw text it stands for.

## Usage

```sh
agent-utils quote -as <shell|json|html|regex> [file]
agent-utils quote -d -as <shell|json|html> [file]
```

## Flags

| Flag           | Description                                                                                   |
| -------------- | --------------------------------------------------------------------------------------------- |
| `-as <target>` | Target syntax: `shell`, `json`, `html`, or `regex`. Required.                                 |
| `-d`           | Decode a literal back to raw text instead of quoting. Works with `shell`, `json`, and `html`. |

Input is taken byte for byte in both directions, so a trailing newline is part of the text and gets quoted with it. `echo` and `<<<` append one; use `printf '%s'` to quote an exact string. The result is printed followed by one newline. With `-d`, the decoded text is printed raw, with no newline added. Whitespace around a `shell` or `json` literal is not part of it, so piping `quote` into `quote -d` gives back the original bytes for those targets; `html` has no delimiters, so a newline after it (from `echo`, or from `quote`'s own output) is decoded as text.

Data a target cannot represent exactly is an error (exit 1), never silently altered: NUL in a shell word or in HTML, invalid UTF-8 for `json`, `html`, and `regex`, and a malformed or non-literal input to `-d`. A missing or unknown `-as`, or `-d -as regex`, is a usage error (exit 2).

### `shell`

One POSIX shell word that evaluates to exactly the input in `sh`, `dash`, `bash`, `ksh`, and `zsh`. The rules are Python's `shlex.quote`: a string made only of ASCII letters, digits, and `@%+=:,./-_` stays bare, the empty string becomes `''`, and anything else is wrapped in single quotes, with each `'` written as `'"'"'`. The one difference is that a leading `=` is quoted too, because zsh expands a bare `=name` to the path of a command. Newlines, invalid UTF-8, and every other byte are allowed; NUL is an error, because no shell argument can hold it.

The result is a word to paste into a command line or a script, such as one run by `sh -c` or `ssh`. It is not needed for an argument to a command you run directly: there, `"$var"` already passes the exact value. A bare result is still subject to its position, so a word like `a=b` is an assignment when it is the first word of a command.

`-d` parses exactly one shell word and prints its value: single quotes, double quotes (where a backslash escapes only `$`, `` ` ``, `"`, `\`, and newline, and is otherwise literal), backslash escapes outside quotes, and backslash-newline line continuations. Blanks (space, tab, newline) around the word are ignored. Anything a shell would not read literally is an error that names the character and its byte offset:

- `$` anywhere outside single quotes: parameter expansion, `$(...)`, `$((...))`, and `$'...'` quoting
- a backtick outside single quotes (command substitution)
- unquoted `*`, `?`, `[` (glob patterns) and `{`, `}` (brace expansion in bash and zsh; zsh also reads a trailing `}` as the end of a command group)
- an unquoted `~` at the start of the word or after an unquoted `=` or `:` (tilde expansion), and an unquoted `=` at the start of the word (zsh)
- an unquoted `#` that starts the word (a comment) and the operators `|`, `&`, `;`, `<`, `>`, `(`, `)`
- a second word, an unterminated quote, a trailing backslash, and NUL

### `json`

A JSON string literal, quotes included. `"`, `\`, and control characters are escaped (`\n`, `\t`, `\u0000`, ...); `<`, `>`, and `&` stay literal, and so does all other Unicode except U+2028 and U+2029, which are escaped as `\u2028` and `\u2029`. Input that is not valid UTF-8 is an error rather than being replaced with U+FFFD.

`-d` takes exactly one JSON string literal (whitespace around it is allowed) and prints its value. Any other JSON value (`null`, a number, an object, ...) is an error that names its type, and so are trailing data, raw control characters inside the string, invalid escapes, and invalid UTF-8. A `\u` escape of a lone UTF-16 surrogate (`"\ud800"`) is an error too, because it is not valid Unicode; many JSON parsers silently turn it into U+FFFD.

### `html`

Text that is safe in HTML text content and in a quoted attribute value: `&`, `<`, `>`, `"`, and `'` become `&amp;`, `&lt;`, `&gt;`, `&#34;`, and `&#39;`. A carriage return becomes `&#13;`, because an HTML parser turns a literal CR or CRLF into LF but keeps a CR written as a character reference. NUL is an error, because HTML parsers drop it or replace it with U+FFFD. The result is not safe inside `<script>`, `<style>`, or a comment, where character references are not decoded, or in an unquoted attribute value.

`-d` decodes every HTML5 named and numeric character reference, following the HTML5 rules for text content, so it is lossy for some inputs that `quote -as html` never produces:

- an unknown entity such as `&nosuch;` is kept as written
- legacy entities are decoded even without a semicolon: `&copy 2026` becomes `© 2026`
- `&#0;`, surrogate code points, and values above U+10FFFF become U+FFFD
- `&#128;` through `&#159;` are read as Windows-1252, so `&#128;` becomes `€`

### `regex`

The input with every regular expression metacharacter escaped (`\.+*?()|[]{}^$`, which is Go's `regexp.QuoteMeta`), so the pattern matches the input literally. It is valid and literal in Go (RE2), PCRE (`grep -P`, `pcre2grep`, Perl), Python's `re`, JavaScript `RegExp` (with or without the `u` and `v` flags, which reject unnecessary escapes), and POSIX extended regular expressions (`grep -E`, `sed -E`). It is not literal:

- in POSIX basic regular expressions (`grep` and `sed` without `-E`), where `\{`, `\(`, and `\|` are operators
- inside a bracket expression (`[...]`), because `-` is not escaped
- with the extended or verbose flag (`x`, `re.VERBOSE`), where whitespace and `#` are significant
- in line-oriented tools such as `grep` and `grep -f`, if the input holds a newline: they read each line of a pattern as a separate pattern
- in a JavaScript regex literal (`/.../`), because `/` is not escaped; use `new RegExp(pattern)`

Input that is not valid UTF-8 is an error, because RE2 rejects it in a pattern. A regular expression does not stand for one raw string, so `-d -as regex` is a usage error.

## Examples

```sh
printf '%s' "it's here" | agent-utils quote -as shell       # 'it'"'"'s here'
printf 'say "hi"\n' | agent-utils quote -as json            # "say \"hi\"\n"
printf '<a href="/">' | agent-utils quote -as html          # &lt;a href=&#34;/&#34;&gt;
printf 'v1.2.*' | agent-utils quote -as regex               # v1\.2\.\*
printf '%s' '"caf\u00e9"' | agent-utils quote -d -as json   # café
printf '%s' "'it'\\''s'" | agent-utils quote -d -as shell   # it's
ssh host "rm -- $(printf '%s' "$file" | agent-utils quote -as shell)"
grep -E "^$(printf '%s' "$version" | agent-utils quote -as regex)\$" versions.txt
```

---

[All commands](../../README.md#commands)
