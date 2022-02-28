package database

import (
	"context"
	"database/sql"
	"log"
	"time"
)

type DbType struct {
	Id    int
	Time  time.Time
	Count int
	Err   bool
}

func ConnectDB(ctx context.Context, db *sql.DB) *sql.DB {
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=db_radio_counter sslmode=disable"
	db, err := sql.Open("pgx", dsn) // *sql.DB
	if err != nil {
		log.Fatalf("failed to load driver: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	return db
}

func InsertDB(ctx context.Context, db *sql.DB, time time.Time, count int, err bool) (flag error) {
	query := `
			INSERT INTO main (date,counter,err)
			VALUES ( $1, $2, $3);
		`
	result, flag := db.Exec(query, time, count, err)
	_ = result
	return
}
