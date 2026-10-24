# `json-schema`

Validate a JSON document against a [JSON Schema](https://json-schema.org), reporting every violation with exactly where and why it fails, as JSON.

## Usage

```sh
agent-utils json-schema <schema-file> [file]
```

The schema is always a file. The document comes from the second argument, or from `stdin` when there is none.

## Output

A pretty-printed JSON report on `stdout`, whether or not the document is valid:

```sh
$ agent-utils json-schema order.schema.json order.json; echo "exit $?"
{
  "valid": false,
  "errors": [
    {
      "instance_path": "/customer/email",
      "schema_path": "/$defs/customer/properties/email/format",
      "message": "'not-an-email' is not valid email: missing @"
    },
    {
      "instance_path": "/items/1/qty",
      "schema_path": "/properties/items/items/properties/qty/exclusiveMinimum",
      "message": "must be > 0, got 0"
    },
    {
      "instance_path": "/items/2",
      "schema_path": "/properties/items/items/required",
      "message": "missing property 'sku'"
    }
  ]
}
agent-utils: json-schema: order.json does not match order.schema.json: 3 errors
exit 3
```

A valid document prints `{"valid": true, "errors": []}` (pretty-printed) and exits `0`. `errors` is always an array, so `jq '.errors[]'` works either way. Each error has:

- `instance_path`: a [JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901) to the failing value in the document. The document itself is `""`, and `/` and `~` in keys are escaped as `~1` and `~0`.
- `schema_path`: a JSON Pointer to the failing keyword in the schema file, the place to look to understand the rule. When `$ref` leads into another document, the pointer is prefixed with that document and `#`: another local file is shown relative to the schema file's directory when that is shorter (`defs/common.json#/$defs/id/type`), a built-in meta-schema by its URL.
- `message`: what is wrong. Strings are quoted with `'`, and numbers are shown exactly as decimals, never rounded through floating point.

Errors are the leaves of the validation: a failing `allOf`, `$ref`, or `if`/`then` reports the failures inside it, not one wrapper error. Under `anyOf` and `oneOf` where no alternative matches, every alternative's failures are listed, and `schema_path` (`/anyOf/0/...`, `/anyOf/1/...`) shows which alternative each belongs to; fixing one alternative is enough. `contains` reports one summary error rather than why each item missed. Errors are sorted by `instance_path` (array indexes in numeric order), then `schema_path`, then `message`, so the same input always gives the same report.

## Exit codes

| Code | Meaning |
| ---- | ------- |
| `0`  | The document is valid. |
| `1`  | The schema or document could not be used: a missing or unreadable file, empty input, malformed JSON, an invalid schema, a reference that cannot be resolved, or a `$ref` loop that the document's validation runs into (the library finds those only while validating, so a loop in a branch the document never reaches goes unnoticed). Nothing is printed on `stdout`. |
| `2`  | Usage error: no schema argument, or more than two arguments. |
| `3`  | The document is valid JSON but does not match the schema. The report is on `stdout` and a one-line summary on `stderr`. |

## Details

**Drafts.** The draft comes from the schema's `$schema`: drafts 4, 6, 7, 2019-09, and 2020-12 are supported, and a schema without `$schema` is read as 2020-12. A schema written for an older draft but missing `$schema` usually fails as an invalid 2020-12 schema (array-form `items`, boolean `exclusiveMaximum`), with a message naming the keyword. Any other `$schema` (draft-03, or a custom meta-schema at a URL) fails with exit `1`.

**Formats are asserted.** `format` is checked in every draft, so `"format": "email"` rejects `"not-an-email"`. In 2019-09 and 2020-12 the specification makes `format` an annotation by default, but a schema author who writes it nearly always means it as a constraint, and an agent checking its own output should learn that a value is malformed. The checked formats are `date-time`, `date`, `time`, `duration`, `period`, `email`, `hostname`, `ipv4`, `ipv6`, `uri`, `uri-reference`, `iri`, `iri-reference`, `uri-template`, `uuid`, `json-pointer`, `relative-json-pointer`, `regex`, and `semver`. Any other format, including `idn-email` and `idn-hostname`, is not checked. `contentEncoding`, `contentMediaType`, and `contentSchema` are annotations and never fail a document.

**No network.** The command never accesses the network. `$ref` (and `$schema`) can point to:

- the schema file itself (`#/$defs/item`) or anything inside it with an `$id` or `$anchor`;
- other local files, resolved relative to the schema file (`defs/common.json`, `defs/common.json#/$defs/id`) or as `file://` URLs;
- the supported drafts' meta-schemas, which are built in (`https://json-schema.org/draft/2020-12/schema`).

Anything else, such as an `http` or `https` URL, or a relative reference under an `$id` that is a web URL, fails with exit `1` and names the reference. To validate against a schema split across remote files, download them and refer to the local copies.

**Regular expressions** in `pattern`, `patternProperties`, and the `regex` format use Go's RE2 syntax rather than ECMAScript's. Common patterns behave the same; lookaheads, lookbehinds, and backreferences are not supported, and a schema using them fails as invalid (exit `1`) instead of being half-checked.

**Strict JSON.** Both the schema and the document must be exactly one JSON value, in UTF-8; a byte order mark at the start of either, or of a file a `$ref` loads, is skipped. Trailing data, invalid UTF-8, and duplicate object keys are errors (exit `1`), each reported with its line and column (a file that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding): with a duplicate key, encoding/json keeps the last value, so validation would check a document other consumers may read differently. Numbers are compared exactly (`9007199254740993` is above a `maximum` of `9007199254740992`, and `0.07` is a `multipleOf` `0.01`), and a number whose decimal exponent is beyond ±1000000 (such as `1e9999999`) is rejected as out of range rather than compared approximately.

**propertyNames.** A `propertyNames` failure names the bad key in its message and points at the object that holds it. The underlying library records that location unreliably, so `json-schema` recovers it from the document; in the rare case where several objects at the same depth hold the same bad key and only some of them are governed by that `propertyNames`, the error points at an enclosing object or array instead.

## Examples

```sh
agent-utils json-schema order.schema.json order.json
agent-utils json-schema schema.json data.json >/dev/null && echo valid
cat response.json | agent-utils json-schema schema.json | jq -r '.errors[] | .instance_path + ": " + .message'
agent-utils yaml2json config.yaml | agent-utils json-schema config.schema.json   # validate YAML
agent-utils json-schema schema.json data.json | jq '.errors | length'            # 0 when valid
```

---

[All commands](../../README.md#commands)
