// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package batchsqlc

import (
	"database/sql/driver"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type StatusEnum string

const (
	StatusEnumQueued  StatusEnum = "queued"
	StatusEnumInprog  StatusEnum = "inprog"
	StatusEnumSuccess StatusEnum = "success"
	StatusEnumFailed  StatusEnum = "failed"
	StatusEnumAborted StatusEnum = "aborted"
	StatusEnumWait    StatusEnum = "wait"
)

func (e *StatusEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = StatusEnum(s)
	case string:
		*e = StatusEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for StatusEnum: %T", src)
	}
	return nil
}

type NullStatusEnum struct {
	StatusEnum StatusEnum `json:"status_enum"`
	Valid      bool       `json:"valid"` // Valid is true if StatusEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullStatusEnum) Scan(value interface{}) error {
	if value == nil {
		ns.StatusEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.StatusEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullStatusEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.StatusEnum), nil
}

type Batch struct {
	ID          uuid.UUID        `json:"id"`
	App         string           `json:"app"`
	Op          string           `json:"op"`
	Context     []byte           `json:"context"`
	Inputfile   pgtype.Text      `json:"inputfile"`
	Status      StatusEnum       `json:"status"`
	Reqat       pgtype.Timestamp `json:"reqat"`
	Doneat      pgtype.Timestamp `json:"doneat"`
	Outputfiles []byte           `json:"outputfiles"`
	Nsuccess    pgtype.Int4      `json:"nsuccess"`
	Nfailed     pgtype.Int4      `json:"nfailed"`
	Naborted    pgtype.Int4      `json:"naborted"`
}

type Batchrow struct {
	Rowid    int32            `json:"rowid"`
	Batch    uuid.UUID        `json:"batch"`
	Line     int32            `json:"line"`
	Input    []byte           `json:"input"`
	Status   StatusEnum       `json:"status"`
	Reqat    pgtype.Timestamp `json:"reqat"`
	Doneat   pgtype.Timestamp `json:"doneat"`
	Res      []byte           `json:"res"`
	Blobrows []byte           `json:"blobrows"`
	Messages []byte           `json:"messages"`
	Doneby   pgtype.Text      `json:"doneby"`
}
