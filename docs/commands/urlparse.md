# `urlparse`

Parse a URL into its components as pretty-printed JSON: exact scheme/host/port/query splitting an agent would otherwise eyeball out of a long string. The URL is a literal argument, or the first line of stdin (trimmed of surrounding whitespace) when no argument is given.

## Usage

```sh
agent-utils urlparse [url]
```

The output schema is fixed: every field is always present, with `""` for absent string components and `{}` for an absent query, so scripts never need existence checks.

| Field | Description |
| ----- | ----------- |
| `scheme` | URL scheme, lowercase (`https`). |
| `opaque` | Scheme-specific part of non-hierarchical URLs (`mailto:a@b.com` puts `a@b.com` here). |
| `user` | Username from the userinfo, percent-decoded. |
| `has_password` | Whether the userinfo carried a password. The password itself is never printed: URLs land in logs and transcripts, and the redaction loses nothing an agent legitimately needs. |
| `host` | Hostname alone: no port, no IPv6 brackets (`[::1]:8080` gives `::1`). |
| `port` | Port as a string, `""` if none; no scheme-default is inferred. |
| `path` | Path, percent-decoded (`/a%20b` gives `/a b`). |
| `raw_query` | Query string exactly as it appeared, undecoded. |
| `query` | Object mapping each key to an array of percent-decoded values, preserving repeated keys in order of appearance. Keys are sorted, so output is deterministic. |
| `fragment` | Fragment without the `#`, percent-decoded. |

Parsing is strict where it matters: an unparseable URL or a malformed percent-escape in the query is an error (exit 1), never a silently dropped component. One `net/url` behavior to know: input without a scheme still parses, but as a bare path: `example.com/x` reports `host: ""` and `path: "example.com/x"`. If you mean a host, say `https://example.com/x`.

## Examples

```sh
agent-utils urlparse 'https://example.com:8443/a%20b?tag=x&tag=y&q=x%26y#frag'
# {
#   "scheme": "https",
#   "opaque": "",
#   "user": "",
#   "has_password": false,
#   "host": "example.com",
#   "port": "8443",
#   "path": "/a b",
#   "raw_query": "tag=x&tag=y&q=x%26y",
#   "query": {
#     "q": [
#       "x&y"
#     ],
#     "tag": [
#       "x",
#       "y"
#     ]
#   },
#   "fragment": "frag"
# }
agent-utils urlparse 'https://example.com/search?q=hello+world' | jq -r '.query.q[0]'   # hello world
pbpaste | agent-utils urlparse | jq -r .host
agent-utils urlparse 'https://user:hunter2@example.com/' | jq .has_password             # true, password never printed
```

---

[All commands](../../README.md#commands)
