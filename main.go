package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
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

func RunEveryHour(ctx context.Context) (int, error) {
	ctx, span := tracing.MakeSpanGet(ctx, "EveryHour")
	defer span.Finish()

	ch := make(chan int)
	count, err := listeners.GetListeners(ctx)

	getCountListeners.Set(float64(count))
	if err == nil {
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
		if Err != nil {
			getError.WithLabelValues("notOK").Inc()
		}
	})
	cr.AddFunc("@every 59m45s", func() { ch <- 1 })
	cr.Start()

	<-ch
	cr.Stop()
	count = count / 4

	return count, err
}

func main() {
	closer := tracing.InitJaeger()
	defer closer.Close()
	ctx := context.Background()

	TimeNow := time.Now()

	var insert = make(chan (database.DbType))
	go database.StartDB(ctx, insert)

	cr := cron.New(cron.WithLocation(time.Now().Location()))
	cr.AddFunc("@hourly", func() {
		TimeNow = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), 0, 0, 0, time.UTC)
		count, err := RunEveryHour(ctx)
		flag := false
		if err != nil {
			flag = true
		}
		database.ZeroRows(ctx, TimeNow.Add(-time.Minute*30), insert)
		fmt.Printf("time : %v, writed : %v, err : %v\n", TimeNow, count, flag)
		insert <- database.DbType{0, TimeNow, count, flag}
	})

	cr.Start()
	metric.RunMetrics()
}
