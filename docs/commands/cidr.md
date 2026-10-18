# `cidr`

CIDR/subnet math for IPv4 and IPv6: exact prefix arithmetic an agent would otherwise approximate. The first argument selects the mode.

## Usage

```sh
agent-utils cidr info <prefix>
agent-utils cidr contains <prefix> <ip>
agent-utils cidr overlaps <prefix> <prefix>
agent-utils cidr split <prefix> <new-length>
```

## Modes

| Mode | Description |
| ---- | ----------- |
| `info <prefix>` | Print the network's details as pretty-printed JSON. |
| `contains <prefix> <ip>` | Is the IP inside the prefix? Print `true`/`false`, exit `0`/`3`. |
| `overlaps <prefix> <prefix>` | Do the two prefixes share any address? Print `true`/`false`, exit `0`/`3`. |
| `split <prefix> <new-length>` | Print the prefix's subnets of the new length, one per line, in address order. |

`info` reports the input as given plus the canonical masked network: `10.0.5.1/24` reports `"network": "10.0.5.0/24"` with `"host_bits_set": true`. Fields: `input`, `network`, `host_bits_set`, `prefix_length`, `first_address`, `last_address`, and `total_addresses`. The total is always a JSON string because IPv6 counts (up to 2^128) overflow every native JSON number; the value is the exact decimal (`"18446744073709551616"` for a /64). Two fields are IPv4-only and omitted for IPv6, which has no dotted netmask notation and no broadcast address: `netmask` (dotted form) and `usable_hosts`. Usable hosts follow convention: prefixes up to /30 exclude the network and broadcast addresses (total minus 2), a /31 has 2 (RFC 3021 point-to-point), and a /32 is a single host route with 1.

Host bits set on a prefix are masked off in every mode. Mixed address families are answered, not rejected: an IPv6 address is never inside an IPv4 prefix and vice versa, so `contains`/`overlaps` print `false` (this includes IPv4-mapped IPv6 forms such as `::ffff:10.0.0.1`, which are IPv6). `split` requires the new length to be between the prefix's own length and the family's width (32 or 128), and refuses to print more than 65536 subnets, reporting the exact count it would have produced. A malformed prefix or address is bad data and exits `1`; a split length that is out of range or would produce too many subnets is a usage error and exits `2`.

## Examples

```sh
agent-utils cidr info 10.0.5.1/24 | jq -r .network      # 10.0.5.0/24
agent-utils cidr info 2001:db8::/64 | jq -r .total_addresses   # 18446744073709551616
agent-utils cidr contains 10.0.0.0/16 10.0.42.1         # true
agent-utils cidr overlaps 10.0.0.0/25 10.0.0.128/25     # false (adjacent, exit 3)
agent-utils cidr split 10.0.0.0/24 26                   # 10.0.0.0/26 ... 10.0.0.192/26, one per line
if agent-utils cidr contains 10.0.0.0/8 "$peer_ip" >/dev/null; then
  echo "peer is internal"
fi
```

---

[All commands](../../README.md#commands)
