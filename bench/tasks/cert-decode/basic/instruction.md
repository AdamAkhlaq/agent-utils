The file `/task/fixtures/chain.pem` holds a TLS certificate chain in PEM form. Write a JSON array to `/out/certs.json` with one object per certificate, in the order the certificates appear in the file, each with exactly these keys:

- `subject_cn`: the common name (CN) of the subject, as a string.
- `issuer_cn`: the common name (CN) of the issuer, as a string.
- `serial`: the serial number in decimal, as a string.
- `not_after`: the end of the validity period in RFC 3339 form in UTC, such as `2030-01-31T23:59:59Z`.
- `dns_names`: the DNS names in the subject alternative name extension, as an array of strings (empty when there are none).
- `sha256_fingerprint`: the SHA-256 fingerprint of the certificate's DER encoding, as 64 lowercase hexadecimal digits with no separators.

Done means `/out/certs.json` parses as JSON and holds exactly those objects. Do not ask questions; write the file and stop.
