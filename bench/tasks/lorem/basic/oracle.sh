#!/bin/sh
set -eu
page=$(cat /task/fixtures/landing.html)
for n in $(grep -o 'data-filler-words="[0-9]*"' /task/fixtures/landing.html | grep -o '[0-9]*'); do
  words=$(agent-utils lorem -w "$n")
  page=$(printf '%s\n' "$page" | sed "s|<p data-filler-words=\"$n\"></p>|<p data-filler-words=\"$n\">$words</p>|")
done
printf '%s\n' "$page" > /out/landing.html
