# `cert-decode`

Decode X.509 certificates (PEM or DER) into a pretty-printed JSON array: answer "why is TLS failing" without wrestling `openssl x509` output. No flags.

## Usage

```sh
agent-utils cert-decode [file]
```

PEM input decodes every `CERTIFICATE` block in order, so a chain file yields one entry per certificate; other block types (a bundled `PRIVATE KEY`, for example) are passed over, but PEM input with zero `CERTIFICATE` blocks is an error naming the block types found. Input without PEM markers is parsed as a single DER certificate. A UTF-8 byte order mark at the start of a PEM file is skipped. Decoding is inspection, not validation: an expired certificate still decodes successfully, with the verdict carried by the `expired` boolean (computed against the current time) and `notAfter`.

Each entry contains `subject` and `issuer` (RFC 2253 style strings), `serialNumber` (decimal string), `notBefore`/`notAfter` (RFC 3339 UTC), `expired`, `selfSigned` (subject equals issuer and the signature verifies against the certificate's own key), the SANs split by type (`dnsNames`, `ipAddresses`, `emailAddresses`, `uris`), `isCA`, `keyUsage` and `extKeyUsage` as readable string arrays, `signatureAlgorithm`, `publicKeyAlgorithm` with `publicKeyBits` (RSA modulus bits, ECDSA curve bits plus a `curve` name, 256 for Ed25519), and `sha256Fingerprint` (lowercase hex, no colons). The output is always an array, even for a single certificate, and the SAN and usage arrays are always present (empty when absent), so the shape is stable for scripts.

## Examples

```sh
agent-utils cert-decode fullchain.pem | jq -r '.[].notAfter'   # expiry across the chain
openssl s_client -connect example.com:443 -showcerts </dev/null 2>/dev/null \
  | agent-utils cert-decode | jq '.[0] | {subject, expired, dnsNames}'
agent-utils cert-decode cert.der                               # DER works too
agent-utils cert-decode chain.pem | jq '[.[] | {subject, selfSigned, isCA}]'
```

---

[All commands](../../README.md#commands)
