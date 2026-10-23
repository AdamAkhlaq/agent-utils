The file `/task/fixtures/tickets.csv` is a CSV export whose first row holds the column names. Write it to `/out/tickets.json` as a JSON array with one object per data row, in file order, mapping each column name to that row's value. Keep every value as a JSON string, exactly as the field holds it once CSV escaping is undone, and an empty field as an empty string.

Done means `/out/tickets.json` parses as JSON and holds exactly those objects. Do not ask questions; write the file and stop.
