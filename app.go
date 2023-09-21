package main

import (
	"context"
	"database/sql"
	"log"
	"reflect"
	"strconv"
	"time"

	querybuilder "go-framework/sql-query/query-builder"

	_ "github.com/lib/pq"
)

// write a function to create connection object
func ConnectDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", "user=postgres password=postgres dbname=gosqlc sslmode=disable")
	if err != nil {
		log.Fatalln("DB connection error:", err)
	}
	return db, nil
}

func run() error {
	ctx := context.Background()

	db, err := ConnectDB()
	if err != nil {
		return err
	}

	queries := querybuilder.New(db)

	// list all authors
	authors, err := queries.ListVouchers(ctx)
	if err != nil {
		return err
	}
	log.Println(authors)

	// create an author
	insertedVoucher, err := queries.CreateVoucher(ctx, querybuilder.CreateVoucherParams{
		Date:            time.Now(),
		DebitAccountID:  sql.NullInt64{Int64: 1, Valid: true},
		CreditAccountID: sql.NullInt64{Int64: 2, Valid: true},
		CostCentreID:    sql.NullInt64{Int64: 3, Valid: true},
		Amount:          sql.NullString{String: "100", Valid: true},
		Narration:       sql.NullString{String: "test", Valid: true},
	})
	if err != nil {
		return err
	}
	log.Println(insertedVoucher)

	// get the author we just inserted
	fetchedVoucher, err := queries.GetVoucher(ctx, insertedVoucher.ID)
	if err != nil {
		return err
	}

	// prints true
	log.Println(reflect.DeepEqual(insertedVoucher, fetchedVoucher))

	//update Voucher details
	err = queries.UpdateVoucher(ctx, querybuilder.UpdateVoucherParams{
		ID:              insertedVoucher.ID,
		Date:            time.Now(),
		DebitAccountID:  sql.NullInt64{Int64: 1, Valid: true},
		CreditAccountID: sql.NullInt64{Int64: 2, Valid: true},
		CostCentreID:    sql.NullInt64{Int64: 3, Valid: true},
		Amount:          sql.NullString{String: "100.00", Valid: true},
		Narration:       sql.NullString{String: "test" + strconv.FormatInt(insertedVoucher.ID, 10), Valid: true},
	})
	if err != nil {
		return err
	}

	log.Println(fetchedVoucher)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
