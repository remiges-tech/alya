// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.22.0

package sqlc

import (
	"database/sql"
	"time"
)

type Employee struct {
	EmployeeID int32          `json:"employee_id" validate:"required"`
	Name       sql.NullString `json:"name"`
	Title      sql.NullString `json:"title"`
	Department sql.NullString `json:"department"`
}

type Voucher struct {
	VoucherID   int32          `json:"voucherId"`
	EmployeeID  int32          `json:"employee_id"`
	DateOfClaim time.Time      `json:"date_of_claim"`
	Amount      float64        `json:"amount"`
	Description sql.NullString `json:"description"`
}
