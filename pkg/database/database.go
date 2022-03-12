package database

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"radio_counter/pkg/listeners"
	"radio_counter/pkg/tracing"
	"time"
)

type DbType struct {
	Id    int
	Time  time.Time
	Count int
	Err   bool
}

var db *sql.DB

func connectDB(ctx context.Context) {
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

	return
}

func StartDB(ctx context.Context, insert chan (DbType)) {
	connectDB(ctx)
	defer db.Close()
	err := db.Ping()
	for {
		select {
		case elem := <-insert:
			for err = insertDB(ctx, db, elem); err != nil; err = insertDB(ctx, db, elem) {
				if db.Ping() != nil {
					db.Close()
					connectDB(ctx)
				}
			}
		}
	}
}

func insertDB(ctx context.Context, db *sql.DB, elem DbType) (flag error) {
	query := `
			INSERT INTO main (date,counter,err)
			VALUES ( $1, $2, $3);
		`
	_, flag = db.Exec(query, elem.Time, elem.Count, elem.Err)
	return
}

func ZeroRows(ctx context.Context, TimeNow time.Time, insert chan (DbType)) error {
	ctx, span := tracing.MakeSpanGet(ctx, "ZeroRows")
	defer span.Finish()
	query := `
		select * from main
		where id = (select max(id) from main)
	`
	rows, err := db.Query(query)
	for err != nil {
		if db.Ping() == nil {
			rows, err = db.Query(query)
		} else {
			db.Close()
			connectDB(ctx)
		}
	}
	defer rows.Close()

	difference := 0
	TimeI := time.Time{}
	middle := 0
	if rows.Next() {
		date := DbType{}
		rows.Scan(&date.Id, &date.Time, &date.Count, &date.Err)

		date.Time = time.Date(date.Time.Year(), date.Time.Month(), date.Time.Day(), date.Time.Hour(), 0, 0, 0, time.UTC)
		TimeNow = time.Date(TimeNow.Year(), TimeNow.Month(), TimeNow.Day(), TimeNow.Hour(), 0, 0, 0, time.UTC)

		TimeI = date.Time.Add(time.Hour * 1)

		difference = int(TimeNow.Sub(date.Time).Hours())
		middle = date.Count
	}
	span.SetTag("Count Hour", difference)

	CountNow, _ := listeners.GetListeners(ctx)
	CountNow = CountNow * 15
	if CountNow == 0 {
		CountNow = middle + middle/10
	}
	rand.Seed(1337)
	for i := 0; i < difference; i++ {
		insert <- DbType{0, TimeI, Min(middle, CountNow) + rand.Intn(Abs(CountNow-middle)), true}
		TimeI = TimeI.Add(time.Hour * 1)
	}

	return nil
}
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func Min(lhs, rhs int) int {
	if lhs < rhs {
		return lhs
	} else {
		return rhs
	}
}
