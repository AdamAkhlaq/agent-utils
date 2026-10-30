The file `/task/fixtures/invoice.csv` lists the lines of an invoice, one per row after the header: a SKU, a description, a quantity, a unit price in dollars, and a discount in percent. A negative quantity is a returned item. Write the invoice's amounts to `/out/invoice.json`:

- Each line's amount is `quantity * unit_price * (100 - discount_pct) / 100`, rounded to the cent.
- The subtotal is the sum of the rounded line amounts.
- The tax is 8.875% of the subtotal, rounded to the cent.
- The total is the subtotal plus the tax.

Round to the nearest cent, and round an amount exactly halfway between two cents away from zero: 2.345 becomes 2.35, and -2.345 becomes -2.35. Write a JSON object with `lines`, an array holding one `{"sku": ..., "amount": ...}` object per row in file order, and `subtotal`, `tax`, and `total`. Write every amount as a JSON string with exactly two decimal places, such as `"-3.10"`.

Done means `/out/invoice.json` parses as JSON and holds exactly those amounts. Do not ask questions; write the file and stop.
