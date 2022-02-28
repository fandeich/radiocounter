package metric

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func RunMetrics() {
	http.Handle("/metric", promhttp.Handler())
	println("listening..")
	http.ListenAndServe(":9100", nil)
}
