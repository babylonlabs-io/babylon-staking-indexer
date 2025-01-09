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

	"github.com/babylonlabs-io/babylon-staking-indexer/internal/utils"
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
	once                           sync.Once
	metricsRouter                  *chi.Mux
	btcClientDurationHistogram     *prometheus.HistogramVec
	queueSendErrorCounter          prometheus.Counter
	clientRequestDurationHistogram *prometheus.HistogramVec
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

	btcClientDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btcclient_duration_seconds",
			Help:    "Histogram of btcclient durations in seconds.",
			Buckets: defaultHistogramBucketsSeconds,
		},
		[]string{"function", "status"},
	)

	// add a counter for the number of errors from the fail to push message into queue
	queueSendErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "queue_send_error_count",
			Help: "The total number of errors when sending messages to the queue",
		},
	)

	prometheus.MustRegister(
		btcClientDurationHistogram,
		queueSendErrorCounter,
		clientRequestDurationHistogram,
	)
}

func RecordBtcClientMetrics[T any](clientRequest func() (T, error)) (T, error) {
	var result T
	functionName := utils.GetFunctionName(1)

	start := time.Now()

	// Perform the client request
	result, err := clientRequest()
	// Determine the outcome status based on whether an error occurred
	status := Success
	if err != nil {
		status = Error
	}

	// Calculate the duration
	duration := time.Since(start).Seconds()

	// Use WithLabelValues to specify the labels and call Observe to record the duration
	btcClientDurationHistogram.WithLabelValues(functionName, status.String()).Observe(duration)

	return result, err
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
