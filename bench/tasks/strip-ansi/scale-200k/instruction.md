The file `/task/fixtures/tests.log` is the output of a test run, captured from a terminal, so it is full of terminal escape sequences: colors and text styles, line erasing, a window title, and hyperlinks. Write it to `/out/tests.log` with every escape sequence removed and nothing else changed: all other bytes, including carriage returns, line breaks, and non-ASCII text, stay exactly as they are, in order. Of each hyperlink, keep only its visible text.

Done means `/out/tests.log` is the original file minus its escape sequences, byte for byte. Do not ask questions; write the file and stop.
