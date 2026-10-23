The product catalog in `/task/fixtures/catalog.txt` was assembled from several systems, so its accented and non-Latin text is stored in a mix of composed and decomposed forms, and lookups against it fail. Write the file converted to Normalization Form C (NFC, canonical decomposition followed by canonical composition) to `/out/catalog.txt`, encoded as UTF-8. Apart from the normalization, keep the text exactly as it is: same lines, same order, same spacing.

Done means `/out/catalog.txt` is exactly the NFC form of the original file. Do not ask questions; write the file and stop.
