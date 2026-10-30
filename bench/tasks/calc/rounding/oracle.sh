#!/bin/sh
set -eu
tail -n +2 /task/fixtures/invoice.csv | {
  sum=0
  lines=
  while IFS=, read -r sku _ qty price discount; do
    amount=$(agent-utils calc -scale 2 -- "$qty * $price * (100 - $discount) / 100")
    sum="$sum + $amount"
    lines="$lines${lines:+,
}    {\"sku\": \"$sku\", \"amount\": \"$amount\"}"
  done
  subtotal=$(agent-utils calc -scale 2 -- "$sum")
  tax=$(agent-utils calc -scale 2 -- "$subtotal * 8.875 / 100")
  total=$(agent-utils calc -scale 2 -- "$subtotal + $tax")
  cat <<EOF
{
  "lines": [
$lines
  ],
  "subtotal": "$subtotal",
  "tax": "$tax",
  "total": "$total"
}
EOF
} > /out/invoice.json
