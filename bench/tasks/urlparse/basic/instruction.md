The file `/task/fixtures/links.txt` holds one web link per line. Break each link into its parts and write a JSON array to `/out/links.json` with one object per line, in file order, each with exactly these keys:

- `scheme`: the scheme in lowercase, such as `https`.
- `host`: the host name or IP address alone, without port, user information, or the square brackets around an IPv6 address.
- `port`: the port as a string, or `""` when the link has none (do not fill in a default).
- `path`: the path, percent-decoded.
- `query`: an object mapping each query parameter name to an array of all its values in order of appearance, names and values decoded the way an HTML form submission encodes them (percent escapes decoded, `+` meaning a space); a parameter with an empty value or no `=` has the value `""`; `{}` when there is no query.
- `fragment`: the fragment without the `#`, percent-decoded, or `""` when there is none.

Done means `/out/links.json` parses as JSON and holds exactly those objects. Do not ask questions; write the file and stop.
