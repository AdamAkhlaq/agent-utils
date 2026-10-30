The file `/task/fixtures/contracts.csv` lists service contracts: an ID, a customer, a start date, and a term in months. Each contract ends exactly its term in calendar months after its start date: on the same day of the month, or on that month's last day when the month is too short to have that day. For example, a contract starting on January 31 with a one-month term ends on the last day of February.

Write `/out/end-dates.csv` with the header `contract_id,end_date` and one row per contract, in file order, giving its end date as `YYYY-MM-DD`.

Done means `/out/end-dates.csv` holds the correct end date of every contract. Do not ask questions; write the file and stop.
