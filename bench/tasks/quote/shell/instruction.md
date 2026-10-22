The file `/task/fixtures/name.txt` holds a file name on its single line (the trailing newline is not part of the name). Write one POSIX shell word that expands to exactly that name, suitable for pasting into a command line run by `sh`, to `/out/word.txt`.

Done means that in both `sh` and `bash`, `set -- <the word>` leaves `$1` equal to the file name and `$#` equal to 1. Do not ask questions; write the file and stop.
