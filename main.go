package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"otel-go-app-example/otelsetup"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	// "go.opentelemetry.io/otel"
	// "go.opentelemetry.io/otel/attribute"
	// "go.opentelemetry.io/otel/codes"
	// "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	// "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	// "go.opentelemetry.io/otel/propagation"
	// "go.opentelemetry.io/otel/sdk/resource"
	// sdktrace "go.opentelemetry.io/otel/sdk/trace"
	// semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	// "go.opentelemetry.io/otel/trace"
	// "google.golang.org/grpc"
	// "google.golang.org/grpc/credentials/insecure"

)

func handleRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Handle database operations
	if err := otelsetup.DatabaseCall(ctx); err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Call external API
	if err := otelsetup.ExternalAPICall(ctx); err != nil {
		http.Error(w, fmt.Sprintf("External API error: %v", err), http.StatusInternalServerError)
		return
	}

	// Success response
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Request processed successfully at %s\n", time.Now().Format(time.RFC3339))
}

func handleSlowAPI(w http.ResponseWriter, r *http.Request) {
	// Simulate slow processing
	time.Sleep(1000 * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Slow API response at %s\n", time.Now().Format(time.RFC3339))
}

func main() {
	// Set service name for Jaeger UI
	os.Setenv("SERVICE_NAME", "otel-go-app-sample-kienlt")
	log.Println("Starting OpenTelemetry example service...")

	// Initialize OpenTelemetry
	shutdown, err := otelsetup.InitProvider()
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}

	// Wrap the handler with OpenTelemetry instrumentation
	handler := http.HandlerFunc(handleRequest)
	wrappedHandler := otelhttp.NewHandler(handler, "http.server.request")

	// Set up an HTTP server
	http.Handle("/", wrappedHandler)

	// Custom health check without tracing
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Slow API endpoint
	http.HandleFunc("/api", handleSlowAPI)

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8883"
	}

	server := &http.Server{
		Addr: ":" + port,
	}

	// Handle graceful shutdown
	go func() {
		log.Printf("Server listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for termination signal
	<-make(chan struct{})

	// Shutdown OpenTelemetry
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown OpenTelemetry provider: %v", err)
	}
}
