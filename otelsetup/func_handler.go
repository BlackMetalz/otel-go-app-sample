package otelsetup

import (
	"fmt"
	"net/http"
	"time"
)

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Handle database operations
	if err := DatabaseCall(ctx); err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Call external API
	if err := ExternalAPICall(ctx); err != nil {
		http.Error(w, fmt.Sprintf("External API error: %v", err), http.StatusInternalServerError)
		return
	}

	// Success response
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Request processed successfully at %s\n", time.Now().Format(time.RFC3339))
}

func HandleSlowAPI(w http.ResponseWriter, r *http.Request) {
	// Simulate slow processing
	time.Sleep(2000 * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Slow API response at %s\n", time.Now().Format(time.RFC3339))
}