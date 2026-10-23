The file `/task/fixtures/orders.json` holds a JSON array of orders, and `/task/fixtures/order.schema.json` is the JSON Schema (draft 2020-12) that every order must satisfy, with its `format` keywords treated as assertions. Find the orders that do not satisfy the schema and write their zero-based positions in the array, as a JSON array of integers in ascending order, to `/out/invalid.json`.

Done means `/out/invalid.json` lists exactly the positions of the invalid orders. Do not ask questions; write the file and stop.
