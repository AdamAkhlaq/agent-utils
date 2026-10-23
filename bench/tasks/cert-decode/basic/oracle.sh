#!/bin/sh
set -eu
agent-utils cert-decode /task/fixtures/chain.pem | jq '[.[] | {
  subject_cn: (.subject | capture("(^|,)CN=(?<cn>[^,]*)").cn),
  issuer_cn: (.issuer | capture("(^|,)CN=(?<cn>[^,]*)").cn),
  serial: .serialNumber,
  not_after: .notAfter,
  dns_names: .dnsNames,
  sha256_fingerprint: .sha256Fingerprint
}]' > /out/certs.json
