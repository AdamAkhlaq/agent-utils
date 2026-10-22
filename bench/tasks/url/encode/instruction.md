The file `/task/fixtures/query.txt` holds a search phrase on its single line (the trailing newline is not part of the phrase). Percent-encode the phrase (RFC 3986, UTF-8) so it can be used as the value of one query-string parameter in an HTTP request, and write the encoded value to `/out/value.txt`.

Done means a standard query-string decoder turns the content of `/out/value.txt` back into the exact phrase, and the value contains no character that would end the parameter or the query. Do not ask questions; write the file and stop.
