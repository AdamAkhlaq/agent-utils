#!/bin/sh
# Pass when /out/result.txt holds the word true: 10.0.5.1/22 is the network
# 10.0.4.0/22 once the host bits are masked, which spans 10.0.4.0 to 10.0.7.255.
if [ ! -f /out/result.txt ]; then
  echo "/out/result.txt was not written" >&2
  exit 1
fi
got=$(tr -d '[:space:]' < /out/result.txt)
if [ "$got" != "true" ]; then
  echo "want true, got \"$got\"" >&2
  exit 1
fi
