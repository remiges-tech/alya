--
-- Employees table
--

-- name: CreateEmployee :one
INSERT INTO Employees (name, title, department)
VALUES ($1, $2, $3)
    RETURNING employee_id;

-- name: GetEmployee :one
SELECT * FROM Employees WHERE employee_id=$1;

-- name: UpdateEmployee :one
UPDATE Employees SET name=$2, title=$3, department=$4 WHERE employee_id=$1
    RETURNING *;

-- name: DeleteEmployee :exec
DELETE FROM Employees WHERE employee_id=$1;

--
-- Vouchers table
--

-- name: CreateVoucher :one
INSERT INTO Vouchers (employee_id, date_of_claim, amount, description)
VALUES ($1, $2, $3, $4)
    RETURNING voucher_id;

-- name: GetVoucher :one
SELECT * FROM Vouchers WHERE voucher_id=$1;


-- name: UpdateVoucher :one
UPDATE Vouchers SET employee_id=$2, date_of_claim=$3, amount=$4, description=$5 WHERE voucher_id=$1
    RETURNING *;

-- name: DeleteVoucher :exec
DELETE FROM Vouchers WHERE voucher_id=$1;


--
-- non-crud queries
--

-- name: GetTotalClaimsPerEmployee :many
SELECT e.employee_id, e.name, e.title, e.department, SUM(v.amount) as total_claims
FROM Employees e
         JOIN Vouchers v
              ON e.employee_id = v.employee_id
GROUP BY e.employee_id, e.name, e.title, e.department;

