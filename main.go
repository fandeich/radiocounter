package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/robfig/cron/v3"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	RadioHeart = "https://a4.radioheart.ru/api/json?userlogin=user8042&api=current_listeners"
	MyRadio24  = "http://myradio24.com/users/meganight/status.json"
)

type dbType struct {
	id    int
	time  time.Time
	count int
	err   bool
}

func GetListenersRadioHeart(name string) (num int, err error) {
	url := RadioHeart
	var netClient = &http.Client{
		Timeout: time.Second * 20,
	}
	res, err := netClient.Get(url)
	if err != nil {
		return -1, err
	}
	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)
	num, _ = strconv.Atoi(result[name].(string))
	return
}

func GetListenersMyRadio24(name string) (num int, err error) {
	url := MyRadio24
	var netClient = &http.Client{
		Timeout: time.Second * 20,
	}
	res, err := netClient.Get(url)
	if err != nil {
		return -1, err
	}
	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)
	num = int(result[name].(float64))
	return
}

func GetListeners() (num int, err error) {
	fmt.Println(time.Now())
	num1, err1 := GetListenersRadioHeart("listeners")
	fmt.Println(time.Now())
	num2, err2 := GetListenersMyRadio24("listeners")
	fmt.Println(time.Now())
	if err1 != nil {
		err = err1
	}
	if err2 != nil {
		err = err2
	}
	num = num1 + num2
	if num < 0 {
		num = 0
	}
	return
}

func RunEveryHour() (count int, err error) {
	ch := make(chan int)
	count, err = GetListeners()
	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@every 1m", func() {
		var n int
		var Err error
		n, Err = GetListeners()

		for n, Err = GetListeners(); Err != nil && time.Now().Second() < 40; {
			n, Err = GetListeners()
			time.Sleep(time.Second * 1)
		}
		count += n
		if err != nil {
			err = Err
		}
	})
	cr.AddFunc("@every 59m5s", func() { ch <- 1 })
	cr.Start()

	<-ch

	count = count / 4
	cr.Stop()
	return
}

func InsertDB(db *sql.DB, time time.Time, count int, err bool) (flag error) {
	query := `
			INSERT INTO main (date,counter,err)
			VALUES ( $1, $2, $3);
		`
	result, flag := db.Exec(query, time, count, err)
	_ = result
	return
}

func ConnectDB() *sql.DB {
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

func ZeroRows(db *sql.DB, TimeNow time.Time) {
	query := `
		select * from main
		where id = (select max(id) from main)
	`
	rows, _ := db.Query(query)
	defer rows.Close()

	difference := 0
	TimeI := time.Time{}
	middle := 0
	if rows.Next() {
		date := dbType{}
		rows.Scan(&date.id, &date.time, &date.count, &date.err)

		date.time = time.Date(date.time.Year(), date.time.Month(), date.time.Day(), date.time.Hour(), 0, 0, 0, time.UTC)
		TimeNow = time.Date(TimeNow.Year(), TimeNow.Month(), TimeNow.Day(), TimeNow.Hour(), 0, 0, 0, time.UTC)

		TimeI = time.Date(date.time.Year(), date.time.Month(), date.time.Day(), date.time.Hour(), 0, 0, 0, time.UTC)
		TimeI = TimeI.Add(time.Hour * 1)

		difference = int(TimeNow.Sub(date.time).Hours())
		middle = date.count
	}

	rand.Seed(1337)
	for i := 0; i < difference; i++ {
		InsertDB(db, TimeI, middle+rand.Intn(50), true)
		TimeI = TimeI.Add(time.Hour * 1)
	}
}

func main() {
	db := ConnectDB()
	defer db.Close()

	TimeNow := time.Now()
	ch := make(chan int)

	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@hourly", func() {
		TimeNow = time.Now()
		count, err := RunEveryHour()
		flag := false

		if err != nil {
			flag = true
		}
		fmt.Printf("time : %v, writed : %v, time : %v\n", TimeNow, count, flag)
		InsertDB(db, time.Date(TimeNow.Year(), TimeNow.Month(), TimeNow.Day(), TimeNow.Hour(), 0, 0, 0, time.UTC), count, flag)
	})

	cr.Start()
	ZeroRows(db, TimeNow)
	<-ch
	cr.Stop()
	fmt.Println("Ending")

}
