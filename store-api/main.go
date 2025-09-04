package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof" // Correct import for HTTP profiling endpoints
	// "os"
	// "time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Global tracer for this service.
var tracer trace.Tracer

const serviceName = "api-service"

// Product represents a product in our system.
type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

// Employee represents an employee in our system.
type Employee struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

// initTracer initializes an OTel tracer provider for the service.
// This example uses a simple stdout exporter, but you would
// configure an OTLP exporter to send traces to Tempo/Grafana.
func initTracer() *sdktrace.TracerProvider {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to initialize stdout exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	tracer = otel.Tracer(serviceName)
	return tp
}

// productsHandler simulates a slow, CPU-intensive endpoint.
func productsHandler(w http.ResponseWriter, r *http.Request) {
	// Create a new span for the handler's logic.
	ctx, span := tracer.Start(r.Context(), "products-handler")
	defer span.End()

	log.Println("Handling /products request...")

	// Simulate a bottleneck to cause a visible spike in the trace.
	// This function will be the target for profiling.
	simulateBottleneck(ctx)

	// Add an attribute to the span to provide more context.
	span.SetAttributes(attribute.Bool("bottleneck_simulated", true))

	products := []Product{
		{ID: "prod-001", Name: "Laptop", Price: 1500},
		{ID: "prod-002", Name: "Mouse", Price: 50},
		{ID: "prod-003", Name: "Keyboard", Price: 120},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
	log.Println("products request handled.")
}

// employeesHandler is a simple, fast endpoint.
func employeesHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling /employees request...")
	employees := []Employee{
		{ID: "emp-001", Name: "Alice", Position: "Engineer"},
		{ID: "emp-002", Name: "Bob", Position: "Manager"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employees)
	log.Println("employees request handled.")
}

// simulateBottleneck is a function that intentionally consumes CPU time.
// This is what will show up in your CPU profile.
func simulateBottleneck(ctx context.Context) {
	// Create a span specifically for the simulated work.
	_, span := tracer.Start(ctx, "simulate-cpu-work")
	defer span.End()

	// Perform a computationally expensive operation.
	// This makes it easy to find in a CPU profile.
	log.Println("Simulating a CPU-intensive bottleneck...")
	var counter int64
	for i := 0; i < 500000000; i++ {
		counter += 1
	}
	fmt.Sprintf("Dummy work result: %d", counter)
	log.Println("CPU-intensive work complete.")
}

func main() {
	// Initialize tracing for this service.
	tp := initTracer()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// Instrument the handlers with OpenTelemetry.
	mux := http.NewServeMux()
	mux.Handle("/products", otelhttp.NewHandler(http.HandlerFunc(productsHandler), "products-handler"))
	mux.Handle("/employees", otelhttp.NewHandler(http.HandlerFunc(employeesHandler), "employees-handler"))

	// Expose pprof endpoints for profiling.
	// Pyroscope or other profilers will scrape these.
	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	port := ":8080"
	log.Printf("Starting %s service on port %s", serviceName, port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
