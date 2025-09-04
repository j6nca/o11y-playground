package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"
	"os"
	"encoding/json"

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
}

type Product struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
}

type Employee struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
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
	}

	// Setup OpenTelemetry for tracing
	shutdown := setupTracer(config)
	defer shutdown()

	// Setup Pyroscope for continuous profiling
	setupProfiler(config)

	// Logger setup for Loki
	slog.Info("Starting Go application...")

	// Define HTTP handlers
	http.Handle("/", otelhttp.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			_, span := otel.Tracer("go.opentelemetry.io/http").Start(ctx, "example-api-handler")
			defer span.End()

			slog.InfoContext(ctx, "Received request on root path", "path", r.URL.Path)

			// Simulating some work
			workDuration := time.Duration(rand.Intn(1000)) * time.Millisecond
			time.Sleep(workDuration)
			workLevel.Set(float64(workDuration.Milliseconds()))

			requestCount.WithLabelValues(r.URL.Path, r.Method).Inc()
			requestLatency.WithLabelValues(r.URL.Path).Observe(workDuration.Seconds())

			slog.InfoContext(ctx, "Request handled successfully", "duration_ms", workDuration.Milliseconds())
			fmt.Fprintf(w, "This is the kitchen store api. Work completed in %d ms.\n", workDuration.Milliseconds())
		}),
		"example-api-handler-span",
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

	http.Handle("/products", otelhttp.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			_, span := otel.Tracer("go.opentelemetry.io/http").Start(ctx, "products-handler")
			defer span.End()

			slog.InfoContext(ctx, "Received request on products path", "path", r.URL.Path)
			start := time.Now()
			products := getProducts()
			duration := time.Since(start)
			requestCount.WithLabelValues(r.URL.Path, r.Method).Inc()
			requestLatency.WithLabelValues(r.URL.Path).Observe(duration.Seconds())

			slog.InfoContext(ctx, "Request handled successfully", "duration_ms", duration.Milliseconds())
			
			jsonData, err := json.Marshal(products)
			if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonData)
		}),
		"products-handler-span",
	))

	http.Handle("/employees", otelhttp.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			_, span := otel.Tracer("go.opentelemetry.io/http").Start(ctx, "employees-handler")
			defer span.End()

			slog.InfoContext(ctx, "Received request on employees path", "path", r.URL.Path)
			start := time.Now()
			employees := getEmployees()
			duration := time.Since(start)
			// For sake of this example, set latency to 0
			requestCount.WithLabelValues(r.URL.Path, r.Method).Inc()
			requestLatency.WithLabelValues(r.URL.Path).Observe(duration.Seconds())

			slog.InfoContext(ctx, "Request handled successfully", "duration_ms", duration.Milliseconds())
			// fmt.Fprintf(w, "Hello, Observability! Work completed in %d ms.\n", workDuration.Milliseconds())

			jsonData, err := json.Marshal(employees)
			if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonData)
		}),
		"employees-handler-span",
	))

	// Endpoint to get metrics
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Application is listening on port 8080...")
	http.ListenAndServe(":8080", nil)
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

func getEmployees() []Employee {
	employees := []Employee{
			{ID: 1, Name: "Jeff"},
			{ID: 2, Name: "Benny"},
			{ID: 3, Name: "Lisa"},
			{ID: 4, Name: "Craig"},
			{ID: 5, Name: "Greg"},
			{ID: 6, Name: "Sheila"},
			{ID: 7, Name: "Steven"},
			{ID: 8, Name: "Kelly"},
			{ID: 9, Name: "Dina"},
			{ID: 10, Name: "Kevin"},
	}
	
	return employees
}

func getProducts() []Product {
	products := []Product{
			{ID: 1, Name: "Mug"},
			{ID: 2, Name: "Bowl"},
			{ID: 3, Name: "Plate"},
			{ID: 4, Name: "Fork"},
			{ID: 5, Name: "Spoon"},
			{ID: 6, Name: "Knife"},
			{ID: 7, Name: "Cup"},
			{ID: 8, Name: "Saucer"},
			{ID: 9, Name: "Dish"},
			{ID: 10, Name: "Glass"},
	}

	cpuIntensiveWork(100000000) // Adjust iterations to control CPU load
	time.Sleep(100 * time.Millisecond) // Add a small delay to avoid 100% CPU saturation
	
	return products
}

// cpuIntensiveWork simulates CPU usage by performing a busy loop.
func cpuIntensiveWork(iterations int) {
	for i := 0; i < iterations; i++ {
		// Perform a simple arithmetic operation to keep the CPU busy
		_ = i * i
	}
}