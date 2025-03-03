package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
	"fmt"

	"otel-go-app-example/otelsetup"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"otel-go-app-example/utils"

	// "github.com/gorilla/mux" // Ensure this is imported
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

func main() {
	// Set service name for Jaeger UI
	os.Setenv("SERVICE_NAME", "otel-go-app-sample-kienlt")
	serviceName := os.Getenv("SERVICE_NAME")
	log.Printf("Starting OpenTelemetry with service name: %s", serviceName)

	// Initialize the database connection
	errMysql := utils.InitDB("kienlt", "123123") // this is just an example, please replace with your own database credentials xD
	if errMysql != nil {
		fmt.Println(errMysql)
		return
	}
	defer utils.DB.Close() // Close the connection when done

	// Initialize OpenTelemetry
	shutdown, err := otelsetup.InitProvider()
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}

	// Set up the Gorilla Mux router
	router := utils.SetupRouter()
	wrappedHandler := otelhttp.NewHandler(router, "http.server.request")

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8883"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: wrappedHandler, // Use the OTel-wrapped router
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
