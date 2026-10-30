# `jwt-decode`

Decode a JWT's header and payload into one pretty-printed JSON document.

## Usage

```sh
agent-utils jwt-decode [file]
```

This only decodes; it does **not** verify the signature, so never treat the claims as authentic on this basis alone. Claim order and number representation are preserved exactly. A leading `Bearer ` (as pasted from an `Authorization` header) is ignored, and the output is a single JSON object, so it pipes straight into `jq`. A header or payload that is not valid base64url, not valid JSON, or not valid UTF-8 is an error (exit 1) naming the segment, so the output is always valid JSON. Output is limited to 64 MiB or 64 times the token's size, whichever is larger, because pretty-printing indents every line by its nesting depth; when the output would be larger, the command fails with exit 1 and an error that names the limit, before anything is written.

## Examples

```sh
printf '%s' "$TOKEN" | agent-utils jwt-decode
agent-utils jwt-decode token.txt
printf '%s' "$TOKEN" | agent-utils jwt-decode | jq -r .payload.exp
```

---

[All commands](../../README.md#commands)
