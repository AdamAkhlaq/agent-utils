The file `/task/fixtures/settings.txt` lists configuration setting names, one per line, written in a mix of styles. Write the environment variable name for each one to `/out/env-names.txt`, one per line, in the same order: the setting's words in uppercase, joined by single underscores.

Split a name into words like this. Any character that is not a letter or a digit (such as `.`, `-`, or `_`) separates words and never appears in the result. A new word also starts at an uppercase letter that follows a lowercase letter or a digit, and at the last uppercase letter of a run of uppercase letters when a lowercase letter follows it, so `HTMLElement` is `HTML` and `Element`. A digit never starts a word: it stays in the word it is part of, so `utf8Name` is `utf8` and `Name`.

Done means `/out/env-names.txt` holds exactly one name per setting, in order. Do not ask questions; write the file and stop.
