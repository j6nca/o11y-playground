package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
	"os"
	"io"

	"github.com/grafana/pyroscope-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"


)

var (
	// Create a new counter vector for total requests.
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_app_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"path", "method"},
	)

	// Create a new histogram for request latencies.
	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "go_app_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)

	// Create a custom gauge for "work" level.
	workLevel = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_app_work_level",
			Help: "Current work level of the application.",
		},
	)
)

type Config struct {
    serviceName string
    pyroscopeServer string
    tempoServer string
		apiServer  string
}

func init() {
	// Register the metrics with Prometheus's default registry.
	prometheus.MustRegister(requestCount, requestLatency, workLevel)
}

func main() {

	config := Config{
		serviceName: os.Getenv("OTEL_SERVICE_NAME"),
		pyroscopeServer: os.Getenv("PYROSCOPE_SERVER_ADDRESS"),
		tempoServer: os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
		apiServer: os.Getenv("API_SERVER_ADDRESS"),
	}

	// Setup OpenTelemetry for tracing
	shutdown := setupTracer(config)
	defer shutdown()

	// Setup Pyroscope for continuous profiling
	setupProfiler(config)

	// Logger setup for Loki
	slog.Info("Starting Kitchen store app ...")

	// Create an HTTP client that automatically adds tracing headers
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	// Define HTTP handlers
	http.Handle("/", otelhttp.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			_, span := otel.Tracer("go.opentelemetry.io/http").Start(ctx, "example-app-handler")
			defer span.End()

			slog.InfoContext(ctx, "Received request on root path", "path", r.URL.Path)

			// Make a request to the first Go service, propagating the trace context
			req, _ := http.NewRequestWithContext(ctx, "GET", config.apiServer, nil)
			resp, err := client.Do(req)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to call Go api service", "error", err)
				http.Error(w, "Failed to call example-api service", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			slog.InfoContext(ctx, "Successfully called Go api service", "status_code", resp.StatusCode)

			// Read and forward the response from the first service
			body, _ := io.ReadAll(resp.Body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write(body)

			requestCount.WithLabelValues(r.URL.Path, r.Method).Inc()
			requestLatency.WithLabelValues(r.URL.Path).Observe(0) // Simplified latency for this example

		}),
		"example-app-handler-span",
	))

	// Path to demonstrate an error
	http.Handle("/error", otelhttp.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Error("An intentional error occurred.", "path", r.URL.Path)
			requestCount.WithLabelValues(r.URL.Path, r.Method).Inc()
			http.Error(w, "An intentional error occurred.", http.StatusInternalServerError)
		}),
		"error-handler-span",
	))

	// Endpoint to get metrics
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Application is listening on port 8081...")
	http.ListenAndServe(":8081", nil)
}

func setupTracer(config Config) func() {
	ctx := context.Background()
	slog.Info("Setting up traces with config", "config", config.tempoServer)
	// Tempo gRPC endpoint from docker-compose.yml
	conn, err := grpc.DialContext(ctx, config.tempoServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		slog.Error("Failed to create gRPC connection to Tempo:", "error", err)
		return func() {}
	}

	// Create a new OTLP gRPC exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		slog.Error("Failed to create a new OTLP exporter:", "error", err)
		return func() {}
	}

	// Create a new tracer provider with the exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.serviceName),
			attribute.String("application", config.serviceName),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown tracer provider:", "error", err)
		}
	}
}

func setupProfiler(config Config) {
	slog.Info("Setting up profiler with config", "config", config.pyroscopeServer)
	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: config.serviceName,
		ServerAddress:   config.pyroscopeServer, // Pyroscope address from docker-compose.yml
		Logger:          pyroscope.StandardLogger,
		// Example tags for profiling data
		Tags: map[string]string{
			"environment": "workshop",
			"service":     config.serviceName,
		},
	})
	if err != nil {
		slog.Error("Failed to start Pyroscope profiler:", "error", err)
	}
}
