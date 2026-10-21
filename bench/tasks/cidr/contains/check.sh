#!/bin/sh
# Pass when /out/result.txt holds the word false: 172.29.141.77/21 is the
# network 172.29.136.0/21 once the host bits are masked, which spans
# 172.29.136.0 to 172.29.143.255, so 172.29.145.6 is outside it. Reasoning
# from the written address (141 plus 8) says it is inside.
if [ ! -f /out/result.txt ]; then
  echo "/out/result.txt was not written" >&2
  exit 1
fi
got=$(tr -d '[:space:]' < /out/result.txt)
if [ "$got" != "false" ]; then
  echo "want false, got \"$got\"" >&2
  exit 1
fi
