# `xml2json`

Convert an XML document to pretty-printed JSON.

## Usage

```sh
agent-utils xml2json [file]
```

XML and JSON do not map onto each other one-to-one, so this command implements one fixed convention:

- An element becomes a JSON object, and the root element's name is the single top-level key.
- Attributes become keys prefixed with `@` (`id="7"` becomes `"@id": "7"`).
- Text content of an element that also has attributes or children goes under `"#text"`. Whitespace-only text between elements is dropped; meaningful text is preserved with leading and trailing whitespace trimmed.
- An element with only text and no attributes becomes a plain JSON string; an empty element becomes `""`.
- Repeated sibling elements with the same name become a JSON array. A single occurrence stays a single value, which is the convention's sharpest edge: a list with one `<item>` produces a string or object where a list with two produces an array, so consumers that expect a list should normalize with something like `jq 'if type == "array" then . else [.] end'`.
- CDATA is treated as text, and entities are decoded. Comments, processing instructions, the XML declaration, and the DOCTYPE are dropped. Namespace prefixes are kept as part of the key name as written (`<x:b>` becomes `"x:b"`, and `xmlns` declarations survive as `@xmlns...` attributes).

Object keys are emitted in sorted order, so the same input always produces byte-identical output. All values are strings; nothing is guessed into numbers or booleans. Malformed XML (unclosed or mismatched tags, text outside the root, multiple roots, unknown entities) fails with a specific error and exit 1. A UTF-8 byte order mark at the start, which XML allows, is skipped. A worked example:

```xml
<rss version="2.0">
  <channel>
    <title>Example</title>
    <item><title>First</title></item>
    <item><title>Second</title></item>
  </channel>
</rss>
```

```json
{
  "rss": {
    "@version": "2.0",
    "channel": {
      "item": [
        {
          "title": "First"
        },
        {
          "title": "Second"
        }
      ],
      "title": "Example"
    }
  }
}
```

## Examples

```sh
agent-utils xml2json pom.xml
curl -s https://example.com/feed.rss | agent-utils xml2json | jq -r '.rss.channel.item[].title'
agent-utils xml2json config.xml | agent-utils json-fmt -c   # minified
```

---

[All commands](../../README.md#commands)
