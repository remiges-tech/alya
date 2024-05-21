Run these programs in the following order:

1. Run `generate_transactions.go` to generate the transactions.csv file containing the transaction data.
2. Run `process_transactions.go` to process the transactions from the CSV file using the batch processor. Updates the balance in Redis at key `batch:balance`.
3. Run `calculate_balance.go` to calculate the final balance from the generated CSV file.

After running these programs, compare the final balance calculated by `calculate_balance.go` with the balance stored in Redis by the batch processor to verify the correctness of the batch processing.

