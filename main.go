package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

var (
	getError = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "get_error",
			Help: "Count of error GetListeners.",
		},
		[]string{"status"},
	)

	getCountListeners = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "get_count_listeners",
			Help: "Count listeners",
		},
	)

	db *sql.DB
)

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

func InsertDB(db *sql.DB, time time.Time, count int, err bool) (flag error) {
	query := `
			INSERT INTO main (date,counter,err)
			VALUES ( $1, $2, $3);
		`
	result, flag := db.Exec(query, time, count, err)
	_ = result
	return
}

func init() {
	prometheus.MustRegister(getError)
	prometheus.MustRegister(getCountListeners)
}

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
	num1, err1 := GetListenersRadioHeart("listeners")
	num2, err2 := GetListenersMyRadio24("listeners")
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

	getCountListeners.Add(float64(count))
	if err == nil {
		getError.WithLabelValues("OK").Inc()
	} else {
		getError.WithLabelValues("notOK").Inc()
	}

	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@every 1m", func() {
		var n int
		var Err error
		n, Err = GetListeners()
		count += n
		if err != nil {
			err = Err
		}

		getCountListeners.Set(float64(n))
		if Err == nil {
			getError.WithLabelValues("OK").Inc()
		} else {
			getError.WithLabelValues("notOK").Inc()
		}
	})
	cr.AddFunc("@every 59m45s", func() { ch <- 1 })
	cr.Start()

	<-ch

	count = count / 4
	cr.Stop()
	return
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
func ZeroRows(TimeNow time.Time) error {
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

		TimeI = date.time.Add(time.Hour * 1)

		difference = int(TimeNow.Sub(date.time).Hours())
		middle = date.count
	}

	CountNow, _ := GetListeners()

	CountNow = CountNow * 15
	rand.Seed(1337)
	for i := 0; i < difference; i++ {
		InsertDB(db, TimeI, Min(middle, CountNow)+rand.Intn(Abs(CountNow-middle)), true)
		TimeI = TimeI.Add(time.Hour * 1)
	}
	return nil
}

func RunMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	println("listening..")
	getError.WithLabelValues("OK").Inc()
	http.ListenAndServe(":9100", nil)
}

func main() {

	for db = ConnectDB(); db == nil; {
		db = ConnectDB()
	}
	defer db.Close()

	TimeNow := time.Now()

	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@hourly", func() {
		TimeNow = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), 0, 0, 0, time.UTC)
		count, err := RunEveryHour()
		flag := false
		if err != nil {
			flag = true
		}
		fmt.Printf("time : %v, writed : %v, err : %v\n", TimeNow, count, flag)
		ZeroRows(TimeNow.Add(-time.Minute * 30))
		InsertDB(db, TimeNow, count, flag)
	})
	cr.Start()

	RunMetrics()
}
