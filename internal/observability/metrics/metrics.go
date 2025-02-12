package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"strconv"
)

type Outcome string

const (
	Success                  Outcome       = "success"
	Error                    Outcome       = "error"
	MetricRequestTimeout     time.Duration = 5 * time.Second
	MetricRequestIdleTimeout time.Duration = 10 * time.Second
)

func (O Outcome) String() string {
	return string(O)
}

var (
	once                                 sync.Once
	metricsRouter                        *chi.Mux
	btcClientLatency                     *prometheus.HistogramVec
	bbnClientLatency                     *prometheus.HistogramVec
	queueSendErrorCounter                prometheus.Counter
	clientRequestDurationHistogram       *prometheus.HistogramVec
	pollerDurationHistogram              *prometheus.HistogramVec
	expiredDelegationsGauge              prometheus.Gauge
	bbnEventProcessingDuration           *prometheus.HistogramVec
	btcNotifierRegisterSpendErrorCounter prometheus.Counter
	btcTipHeightGauge                    prometheus.Gauge
	dbLatency                            *prometheus.HistogramVec
)

// Init initializes the metrics package.
func Init(metricsPort int) {
	once.Do(func() {
		initMetricsRouter(metricsPort)
		registerMetrics()
	})
}

// initMetricsRouter initializes the metrics router.
func initMetricsRouter(metricsPort int) {
	metricsRouter = chi.NewRouter()
	metricsRouter.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
	// Create a custom server with timeout settings
	metricsAddr := fmt.Sprintf(":%d", metricsPort)
	server := &http.Server{
		Addr:         metricsAddr,
		Handler:      metricsRouter,
		ReadTimeout:  MetricRequestTimeout,
		WriteTimeout: MetricRequestTimeout,
		IdleTimeout:  MetricRequestIdleTimeout,
	}

	// Start the server in a separate goroutine
	go func() {
		log.Printf("Starting metrics server on %s", metricsAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msgf("Error starting metrics server on %s", metricsAddr)
		}
	}()
}

// registerMetrics initializes and register the Prometheus metrics.
func registerMetrics() {
	defaultHistogramBucketsSeconds := []float64{0.1, 0.5, 1, 2.5, 5, 10, 30}

	// client requests are the ones sending to other service
	clientRequestDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "client_request_duration_seconds",
			Help:    "Histogram of outgoing client request durations in seconds.",
			Buckets: defaultHistogramBucketsSeconds,
		},
		[]string{"baseurl", "method", "path", "status"},
	)

	btcClientLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_client_latency_seconds",
			Help:    "Histogram of btc client durations in seconds.",
			Buckets: defaultHistogramBucketsSeconds,
		},
		[]string{"method", "status"},
	)

	bbnClientLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bbn_client_latency_seconds",
			Help:    "Histogram of bbn client durations in seconds.",
			Buckets: defaultHistogramBucketsSeconds,
		},
		[]string{"method", "status"},
	)

	// add a counter for the number of errors from the fail to push message into queue
	queueSendErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "queue_send_error_count",
			Help: "The total number of errors when sending messages to the queue",
		},
	)

	pollerDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "poller_duration_seconds",
			Help:    "Histogram of poller durations in seconds.",
			Buckets: defaultHistogramBucketsSeconds,
		},
		[]string{"type", "status"},
	)

	expiredDelegationsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "expired_delegations_count",
			Help: "Number of expired delegations",
		},
	)

	bbnEventProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bbn_event_processing_duration_seconds",
			Help:    "BBN event processing duration in seconds.",
			Buckets: defaultHistogramBucketsSeconds,
		},
		[]string{"event_type", "status", "retry"},
	)

	btcNotifierRegisterSpendErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "btc_notifier_register_spend_error_count",
			Help: "Number of failures in btcNotifier.RegisterSpendNtfn() calls",
		},
	)

	btcTipHeightGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "btc_tip_height",
			Help: "Last value of btc height retrieved",
		},
	)

	dbLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "db_latency_seconds",
			Help: "DB latency in seconds splitted by method and execution status",
		},
		[]string{"method", "status"},
	)

	prometheus.MustRegister(
		btcClientLatency,
		bbnClientLatency,
		queueSendErrorCounter,
		clientRequestDurationHistogram,
		pollerDurationHistogram,
		expiredDelegationsGauge,
		bbnEventProcessingDuration,
		btcNotifierRegisterSpendErrorCounter,
		btcTipHeightGauge,
		dbLatency,
	)
}

func RecordBTCClientLatency(d time.Duration, method string, failure bool) {
	status := Success
	if failure {
		status = Error
	}

	btcClientLatency.WithLabelValues(method, status.String()).Observe(d.Seconds())
}

func RecordBBNClientLatency(d time.Duration, method string, failure bool) {
	status := Success
	if failure {
		status = Error
	}

	btcClientLatency.WithLabelValues(method, status.String()).Observe(d.Seconds())
}

func RecordDbLatency(d time.Duration, method string, failure bool) {
	status := Success
	if failure {
		status = Error
	}

	dbLatency.WithLabelValues(method, status.String()).Observe(d.Seconds())
}

func RecordBtcTipHeight(height uint64) {
	btcTipHeightGauge.Set(float64(height))
}

func IncBtcNotifierRegisterSpendFailures() {
	btcNotifierRegisterSpendErrorCounter.Inc()
}

func RecordExpiredDelegationsCount(count int) {
	expiredDelegationsGauge.Set(float64(count))
}

func RecordBbnEventProcessingDuration(d time.Duration, eventType string, retry int, failure bool) {
	status := Success
	if failure {
		status = Error
	}

	retryStr := strconv.Itoa(retry)

	bbnEventProcessingDuration.WithLabelValues(eventType, status.String(), retryStr).Observe(d.Seconds())
}

// StartClientRequestDurationTimer starts a timer to measure outgoing client request duration.
func StartClientRequestDurationTimer(baseUrl, method, path string) func(statusCode int) {
	startTime := time.Now()
	return func(statusCode int) {
		duration := time.Since(startTime).Seconds()
		clientRequestDurationHistogram.WithLabelValues(
			baseUrl,
			method,
			path,
			fmt.Sprintf("%d", statusCode),
		).Observe(duration)
	}
}

func RecordQueueSendError() {
	queueSendErrorCounter.Inc()
}
