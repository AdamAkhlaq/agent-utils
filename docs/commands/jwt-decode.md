# `jwt-decode`

Decode a JWT's header and payload into one pretty-printed JSON document.

## Usage

```sh
agent-utils jwt-decode [file]
```

This only decodes; it does **not** verify the signature, so never treat the claims as authentic on this basis alone. Claim order and number representation are preserved exactly. A leading `Bearer ` (as pasted from an `Authorization` header) is ignored, and the output is a single JSON object, so it pipes straight into `jq`.

## Examples

```sh
printf '%s' "$TOKEN" | agent-utils jwt-decode
agent-utils jwt-decode token.txt
printf '%s' "$TOKEN" | agent-utils jwt-decode | jq -r .payload.exp
```

---

[All commands](../../README.md#commands)
