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
	"runtime/pprof"

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
	Price int   `json:"price"`
}

type Employee struct {
	ID   int    		`json:"id"`
	Name string 		`json:"name"`
	Position string `json:"position"`
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

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Setup OpenTelemetry for tracing
	shutdown := setupTracer(config)
	defer shutdown()

	// Setup Pyroscope for continuous profiling
	setupProfiler(config)

	// Logger setup for Loki
	slog.Info("Starting Kitchen Store API application...")

	// Instrument the handlers with OpenTelemetry.
	mux := http.NewServeMux()
	mux.Handle("/products", otelhttp.NewHandler(http.HandlerFunc(productsHandler), "products-handler"))
	mux.Handle("/employees", otelhttp.NewHandler(http.HandlerFunc(employeesHandler), "employees-handler"))

	// Expose pprof endpoints for profiling.
	// Pyroscope or other profilers will scrape these.
	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	
	port := ":8080"
	slog.Info("Application is listening on port 8080...")
	http.ListenAndServe(port, mux)
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

// employeesHandler is a simple, fast endpoint.
func employeesHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Handling /employees request...")
	employees := []Employee{
			{ID: 1, Name: "Jeff", Position: "Manager"},
			{ID: 2, Name: "Benny", Position: "Sales Associate"},
			{ID: 3, Name: "Lisa", Position: "Assistant Manager"},
			{ID: 4, Name: "Craig", Position: "Sales Associate"},
			{ID: 5, Name: "Greg", Position: "Sales Associate"},
			{ID: 6, Name: "Sheila", Position: "Product Tester"},
			{ID: 7, Name: "Steven", Position: "Clerk"},
			{ID: 8, Name: "Kelly", Position: "Clerk"},
			{ID: 9, Name: "Dina", Position: "Cashier"},
			{ID: 10, Name: "Kevin", Position: "Cashier"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employees)
	slog.Info("employees request handled.")
}

// productsHandler simulates a slow, CPU-intensive endpoint.
func productsHandler(w http.ResponseWriter, r *http.Request) {
	// Create a new span for the handler's logic.
	ctx, span := tracer.Start(r.Context(), "products-handler")
	defer span.End()

	slog.Info("Handling /products request...")

	// Simulate a bottleneck to cause a visible spike in the trace.
	// This function will be the target for profiling.
	simulateBottleneck(ctx)

	// Add an attribute to the span to provide more context.
	span.SetAttributes(attribute.Bool("bottleneck_simulated", true))

	products := []Product{
			{ID: 1, Name: "Mug", Price: 1099},
			{ID: 2, Name: "Bowl", Price: 1599},
			{ID: 3, Name: "Plate", Price: 1299},
			{ID: 4, Name: "Fork", Price: 599},
			{ID: 5, Name: "Spoon", Price: 799},
			{ID: 6, Name: "Knife", Price: 1099},
			{ID: 7, Name: "Cup", Price: 899},
			{ID: 8, Name: "Saucer", Price: 699},
			{ID: 9, Name: "Dish", Price: 1499},
			{ID: 10, Name: "Glass", Price: 1199},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
	slog.Info("products request handled.")
}

// simulateBottleneck is a function that intentionally consumes CPU time.
// This is what will show up in your CPU profile.
func simulateBottleneck(ctx context.Context) {
	// Create a span specifically for the simulated work.
	_, span := tracer.Start(ctx, "simulate-cpu-work")
	defer span.End()

	// Perform a computationally expensive operation.
	// This makes it easy to find in a CPU profile.
	slog.Info("Simulating a CPU-intensive bottleneck...")
	var counter int64
	for i := 0; i < 5000000000; i++ {
		counter += 1
	}
	fmt.Sprintf("Dummy work result: %d", counter)
	slog.Info("CPU-intensive work complete.")
}