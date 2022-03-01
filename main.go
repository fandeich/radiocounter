package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
	"math/rand"
	"radio_counter/pkg/database"
	"radio_counter/pkg/listeners"
	"radio_counter/pkg/metric"
	"radio_counter/pkg/tracing"
	"time"
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

func init() {
	prometheus.MustRegister(getError)
	prometheus.MustRegister(getCountListeners)
}

func RunEveryHour(ctx context.Context) (count int, err error) {
	ctx, span := tracing.MakeSpanGet(ctx, "EveryHour")
	defer span.Finish()

	ch := make(chan int)
	count, err = listeners.GetListeners(ctx)

	getCountListeners.Set(float64(count))
	if err == nil {
		getError.WithLabelValues("OK").Inc()
	} else {
		getError.WithLabelValues("notOK").Inc()
	}

	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@every 1m", func() {
		var n int
		var Err error
		n, Err = listeners.GetListeners(ctx)
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
func ZeroRows(ctx context.Context, TimeNow time.Time) error {
	ctx, span := tracing.MakeSpanGet(ctx, "ZeroRows")
	defer span.Finish()

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
		date := database.DbType{}
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
		return nil
	}
	rand.Seed(1337)
	for i := 0; i < difference; i++ {
		database.InsertDB(ctx, db, TimeI, Min(middle, CountNow)+rand.Intn(Abs(CountNow-middle)), true)
		TimeI = TimeI.Add(time.Hour * 1)
	}

	return nil
}

func main() {
	closer := tracing.InitJaeger()
	defer closer.Close()

	ctx := context.Background()

	TimeNow := time.Now()

	for db = database.ConnectDB(ctx, db); db == nil; {
		db = database.ConnectDB(ctx, db)
	}
	defer db.Close()
	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@hourly", func() {
		TimeNow = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), 0, 0, 0, time.UTC)
		count, err := RunEveryHour(ctx)
		flag := false
		if err != nil {
			flag = true
		}
		fmt.Printf("time : %v, writed : %v, err : %v\n", TimeNow, count, flag)
		ZeroRows(ctx, TimeNow.Add(-time.Minute*30))
		if count != 0 {
			database.InsertDB(ctx, db, TimeNow, count, flag)
		}
	})
	cr.Start()

	metric.RunMetrics()

}
