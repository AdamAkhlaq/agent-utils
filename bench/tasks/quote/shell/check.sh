#!/bin/sh
# Pass when /out/word.txt holds one shell word that both sh (dash) and bash
# expand to the file name: "set -- <word>" must leave exactly one positional
# parameter equal to the name. Any correct quoting style passes (single
# quotes, double quotes with escapes, or backslashes).
if [ ! -f /out/word.txt ]; then
  echo "/out/word.txt was not written" >&2
  exit 1
fi
want=$(cat /task/fixtures/name.txt)
word=$(cat /out/word.txt)
if [ -z "$word" ]; then
  echo "/out/word.txt is empty" >&2
  exit 1
fi
for shell in sh bash; do
  # The "|" sentinel keeps a trailing newline in the expansion visible,
  # since $(...) would strip it and hide a word that quoted the newline.
  got=$("$shell" -c 'eval "set -- $1" || exit 9; [ "$#" -eq 1 ] || { echo "$#" >&2; exit 8; }; printf "%s|" "$1"' "$shell" "$word" 2>/tmp/err)
  rc=$?
  got=${got%"|"}
  case $rc in
    0) ;;
    9) echo "$shell could not parse the word: $(cat /tmp/err)" >&2; exit 1 ;;
    8) echo "$shell expands the word to $(cat /tmp/err) words, not 1" >&2; exit 1 ;;
    *) echo "$shell failed with exit $rc: $(cat /tmp/err)" >&2; exit 1 ;;
  esac
  if [ "$got" != "$want" ]; then
    printf '%s expands the word to the wrong name\nwant: %s\ngot:  %s\n' "$shell" "$want" "$got" >&2
    exit 1
  fi
done
