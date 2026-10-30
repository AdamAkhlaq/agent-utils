# `yaml2json`

Convert a YAML document to pretty-printed JSON.

## Usage

```sh
agent-utils yaml2json [file]
```

Key order is preserved, and only `true` and `false` are booleans (`yes`/`no` stay strings). Input with more than one document is rejected rather than silently truncated, an empty `---` document included. A `%YAML 1.1` or `%YAML 1.2` directive is accepted, and the document after it is read by the rules below either way (so `yes` stays a string under `%YAML 1.1` too); other directives, `%TAG` included, are an error. A UTF-8 byte order mark at the start of the input is skipped, since YAML does not count it as content. Input must be UTF-8: an invalid byte is an error that names its line and column rather than being replaced with U+FFFD, and input that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding. Output pipes straight into `jq` or `json-fmt`. Every error below exits with code `1`, and nothing is written to `stdout` unless the whole document converts.

## Details

**Scalars.** Integers keep their exact value up to the unsigned 64-bit range (`12345678901234567890` stays as written); hex (`0x1F`), octal (`0o17`, and also `017` with a leading zero), and binary (`0b11`) integers are written in decimal, and integers too large for 64 bits become strings. Floats are written in the shortest form that reads back as the same 64-bit float, always with a decimal point or an exponent (`1.0` stays `1.0`, `1.5e3` becomes `1500.0`); an exponent without a decimal point (`1e3`) and a float too large for 64 bits (`1e400`) are read as strings. `~`, `null`, and an empty value become `null`. Strings are escaped as JSON requires, with `\uXXXX` for control characters and other characters that do not print. `.inf`, `-.inf`, and `.nan` have no JSON form and are an error.

**Keys.** JSON object keys are strings, so other scalar keys are converted: `1: a` gives `"1"`, `1.0: a` gives `"1"`, `true: a` gives `"true"`, `~: a` gives `"null"`, a `!!timestamp` key is written in RFC 3339 as it is as a value, and a `!!binary` key as base64. Two keys in one mapping that convert to the same string (such as `1` and `1.0`) are an error rather than a duplicate key in the output, and so is an alias used as a key that refers to a mapping or a sequence.

**Tags.** `!!str` keeps the scalar exactly as written (`!!str 0x1F` gives `"0x1F"`, `!!str ~` gives `"~"`). `!!int`, `!!float`, and `!!bool` convert the scalar they tag (`!!int "7"` gives `7`, and `!!int` keeps the unsigned 64-bit range), `!!null` accepts only an empty value, `~`, `null`, `Null`, or `NULL`, `!!timestamp` gives an RFC 3339 string (`!!timestamp 2001-12-14` gives `"2001-12-14T00:00:00Z"`), and `!!binary` decodes the base64 text as written, ignoring whitespace, into an array of byte values (`!!binary 1234` is base64 too). A value a tag cannot convert, such as `!!int 1.5`, `!!int` past the 64-bit range, `!!float abc`, `!!null x`, or `!!binary` text that is not base64, is an error, and so is a scalar tag on a mapping or a sequence. Other tags, custom ones included, leave the value as it is. A node has at most one tag and one anchor, as YAML requires: `!!str !!str x` is an error (go-yaml accepts it, and with three tags drops the rest of the mapping). go-yaml also misreads an empty value that has a tag when another key follows it (`a: !!str &x`, then `b: 1`), so that is an error too; write the value explicitly. A tag on the last line of input with no line break after it is read as if one followed (go-yaml drops it otherwise, so `a: !!str` gives `""` and `a: !!int` is an error).

**Anchors and aliases.** An alias (`*name`) stands for a copy of the node its anchor (`&name`) marks: the most recent anchor of that name earlier in the document, as written, with the node's tag (`!!str &x 5` and `&x !!str 5` both make `*x` the string `"5"`). A quoted name matches only the same quoting, so `&"x"` and `*x` are different names. An alias with no earlier anchor, inside the node its own anchor marks (which would make the document infinite), or with a tag of its own (`!!str *x`, which YAML does not allow), is an error.

**Merge keys.** A `<<` key merges a mapping, or a list of mappings, into the mapping that holds it, following the YAML merge-key spec. A key written in the mapping itself wins over a merged key wherever the `<<` appears, and an earlier mapping in a merge list wins over a later one. Every key appears once in the output: explicit keys where they are written, merged keys where the `<<` is. A merged value that another key overrides is not converted, so it is not checked either. A `<<` whose value is anything else, null included, is an error.

**Limits.** Input is untrusted, so the work it can cause is capped, and input over a cap fails with an error that names it:

- **Alias expansion: 250,000 nodes and 16 MiB of output.** A node is one value or key, or one mapping named by a merge key, counted each time an alias brings it into the output (an alias inside an alias counts at every level, and a merged key that loses to an explicit one still counts). Each byte of a `!!binary` value counts as a node. Output is the bytes of pretty-printed JSON that aliases write, indentation included; a `!!binary` value is checked against it before it is written. A scalar an alias brings in is decoded once, however many aliases repeat it, so an alias costs time in proportion to what it writes. Content written without going through an alias does not count toward this limit; the output limit below covers the whole document. A "billion laughs" file of 414 bytes, whose nested aliases would expand to gigabytes, fails at once with `alias expansion exceeds the size limit`; the most alias-heavy real config measured, GitLab's `rules.gitlab-ci.yml`, adds 8,873 nodes and 389 KiB.
- **Nesting: 10,000 levels.** Mappings and sequences nested deeper than 10,000 levels, counting what aliases bring in, are an error, and so are merge keys nested that deep through aliases. Nesting written with brackets or with several `- ` entries on one line is checked before parsing, so a short file of millions of `[` fails at once.
- **Output: 64 MiB or 64 times the input size, whichever is larger.** Pretty-printing indents every line by its nesting depth, so 20 KB of brackets nested 9,999 levels deep would print 200 MB. The lines a document must write are counted before any output is built, so input like that fails at once; what the count leaves out (scalar text, and what aliases bring in) is checked as it is written. Either way, nothing is written to `stdout`.

## Examples

```sh
agent-utils yaml2json config.yaml
cat config.yaml | agent-utils yaml2json | jq .service.name
agent-utils yaml2json config.yaml | agent-utils json-fmt -c   # minified
printf 'base: &base {image: app, restart: always}\nweb: {<<: *base, restart: "no"}\n' | agent-utils yaml2json | jq -c .web
# {"image":"app","restart":"no"}
```

---

[All commands](../../README.md#commands)
