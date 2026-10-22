#!/bin/sh
set -eu
# xml2json gives a lone element as an object and repeated ones as an array,
# so both levels are normalized to arrays before selecting.
agent-utils xml2json /task/fixtures/report.xml | jq '[
  .testsuites.testsuite | (if type == "array" then . else [.] end)[]
  | .["@name"] as $suite
  | (.testcase | if type == "array" then . else [.] end)[]
  | select(.failure or .error)
  | {suite: $suite, name: .["@name"], message: (.failure // .error)["@message"]}
]' > /out/failures.json
