-- name: GetVoucher :one
SELECT * FROM voucher WHERE id = $1 LIMIT 1;

-- name: ListVouchers :many
SELECT * FROM voucher;

-- name: CreateVoucher :one
INSERT INTO voucher (date, debit_account_id, credit_account_id, cost_centre_id, amount, narration)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateVoucher :exec 
UPDATE voucher
  set date = $2,
  debit_account_id = $3,
  credit_account_id = $4,
  cost_centre_id = $5,
  amount = $6,
  narration = $7
WHERE id = $1;
