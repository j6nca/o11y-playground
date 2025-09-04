package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	// "time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Global tracer for this service.
var tracer trace.Tracer

const serviceName = "client-service"
const apiServiceURL = "http://store-api:8080"

// Product represents a product in our system.
// This is needed to unmarshal the JSON response from the API service.
type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

// initTracer initializes an OTel tracer provider for the service.
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

func handleLandingPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, span := otel.Tracer("go.opentelemetry.io/http").Start(ctx, "example-api-handler")
	defer span.End()

	log.Println("Client request received, serving landing page...")
	fmt.Fprintln(w, "Welcome to the kitchen store!")
}

// fetchProducts makes a request to the backend API and presents the data.
func fetchProducts(w http.ResponseWriter, r *http.Request) {
	// Create a span for the entire request.
	_, span := tracer.Start(r.Context(), "client-products-page")
	defer span.End()

	log.Println("Client request received, fetching products from API...")

	// Make a request to the API service's products endpoint.
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	resp, err := client.Get(fmt.Sprintf("%s/products", apiServiceURL))
	if err != nil {
		span.RecordError(err)
		http.Error(w, fmt.Sprintf("Error fetching products: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("API service returned non-200 status code: %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	var products []Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		span.RecordError(err)
		http.Error(w, fmt.Sprintf("Error decoding products JSON: %v", err), http.StatusInternalServerError)
		return
	}

	// Format the product data into a user-friendly response.
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>Our Products</h1><ul>")
	for _, p := range products {
		fmt.Fprintf(w, "<li><strong>%s</strong>: %s ($%d)</li>", p.ID, p.Name, p.Price)
	}
	fmt.Fprintf(w, "</ul></body></html>")

	log.Println("Products data served to client.")
}

func main() {
	// Initialize tracing for the client.
	tp := initTracer()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// Instrument the handler with OpenTelemetry.
	mux := http.NewServeMux()
	mux.Handle("/", otelhttp.NewHandler(http.HandlerFunc(handleLandingPage), "/"))
	mux.Handle("/products", otelhttp.NewHandler(http.HandlerFunc(fetchProducts), "/products"))

	port := ":8081"
	log.Printf("Starting %s service on port %s", serviceName, port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
