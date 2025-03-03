package utils

import (
	"database/sql"
	"fmt"
	"net/http"
	"encoding/json"
	"time"
	"math/rand"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux" // Included as per your import, though not used in this specific code
	"otel-go-app-example/otelsetup" // Import otelsetup to access HandleSlowAPI
	"net/http/httptest" // Add this import

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	// "go.opentelemetry.io/otel/trace"
	"context" // Add this import
)

// Product represents the structure of the products table
type Product struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float32 `json:"price"`
}

// Response combines products and slow API status
type Response struct {
	Products      []Product `json:"products"`
	SlowStatus    int       `json:"slow_status"`
	SlowMessage   string    `json:"slow_message"`
	ProcessStatus string    `json:"process_status"`
}

// DB is a global variable to hold the database connection (optional, can be passed as a parameter instead)
var DB *sql.DB

// InitDB initializes the database connection
func InitDB(username, password string) error {
	// Data Source Name (DSN) format: username:password@tcp(host:port)/dbname?charset=utf8
	dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/inventory?charset=utf8", username, password)
	
	// Open database connection
	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	// Test the connection
	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("error connecting to the database: %v", err)
	}

	fmt.Println("Successfully connected to the MySQL database!")
	return nil
}

// GetAllProducts retrieves all records from the products table
func GetAllProducts(ctx context.Context) ([]Product, error) {
	_, span := otel.Tracer("utils").Start(ctx, "GetAllProducts")
	defer span.End()

	if DB == nil {
		span.SetAttributes(attribute.String("error", "database not initialized"))
		span.End()
		return nil, fmt.Errorf("database connection not initialized; call InitDB first")
	}
	query := "SELECT id, name, quantity, price FROM products"
	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("error querying products: %v", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		err := rows.Scan(&p.ID, &p.Name, &p.Quantity, &p.Price)
		if err != nil {
			span.SetAttributes(attribute.String("error", err.Error()))
			return nil, fmt.Errorf("error scanning product row: %v", err)
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("error iterating over product rows: %v", err)
	}
	span.SetAttributes(attribute.Int("product_count", len(products)))
	return products, nil
}

// validateRequest simulates a validation step with a small delay
func validateRequest(ctx context.Context) error {
	_, span := otel.Tracer("utils").Start(ctx, "validateRequest")
	defer span.End()

	time.Sleep(1000 * time.Millisecond) // Small delay (1s or 100ms your choice)
	if rand.Float32() < 0.2 {          // 20% chance of failure
		span.SetAttributes(attribute.String("error", "validation failed"))
		return fmt.Errorf("request validation failed")
	}
	return nil
}

// processData simulates additional processing with a delay and possible error
func processData(ctx context.Context, products []Product) (string, error) {
	_, span := otel.Tracer("utils").Start(ctx, "processData")
	defer span.End()

	time.Sleep(1500 * time.Millisecond) // Medium delay
	if rand.Float32() < 0.3 {          // 30% chance of failure
		span.SetAttributes(attribute.String("error", "processing failed"))
		return "failed", fmt.Errorf("data processing failed")
	}
	span.SetAttributes(attribute.Int("processed_count", len(products)))
	return "success", nil
}

// GetProductsHandler handles the /products endpoint with a complex accident scenario
func GetProductsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("utils")
	_, span := tracer.Start(ctx, "GetProductsHandler")
	defer span.End()

	// Step 1: Validate the request
	if err := validateRequest(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		span.SetAttributes(attribute.String("error", err.Error()))
		return
	}

	// Step 2: Fetch products from the database
	products, err := GetAllProducts(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch products: %v", err), http.StatusInternalServerError)
		span.SetAttributes(attribute.String("error", err.Error()))
		return
	}

	// Step 3: Call HandleSlowAPI in parallel with processing
	type slowResult struct {
		status int
		msg    string
	}
	slowChan := make(chan slowResult)
	go func() {
		recorder := httptest.NewRecorder()
		otelsetup.HandleSlowAPI(recorder, r.WithContext(ctx)) // Pass the traced context
		slowChan <- slowResult{status: recorder.Code, msg: recorder.Body.String()}
	}()

	// Step 4: Process the data concurrently
	processStatus, err := processData(ctx, products)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process data: %v", err), http.StatusInternalServerError)
		span.SetAttributes(attribute.String("error", err.Error()))
		return
	}

	// Step 5: Wait for the slow API result
	slow := <-slowChan

	// Prepare the combined response
	resp := Response{
		Products:     products,
		SlowStatus:   slow.status,
		SlowMessage:  slow.msg,
		ProcessStatus: processStatus,
	}

	// Set response header and write JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// SetupRouter configures the Gorilla Mux router with all endpoints
func SetupRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", otelsetup.HandleRequest).Methods("GET")
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
	r.HandleFunc("/api", otelsetup.HandleSlowAPI).Methods("GET")
	r.HandleFunc("/products", GetProductsHandler).Methods("GET")
	return r
}

/*
// Get all products
products, err := utils.GetAllProducts()
if err != nil {
	fmt.Println(err)
	return
}

// Print the results
for _, p := range products {
	fmt.Printf("ID: %d, Name: %s, Quantity: %d, Price: %.2f\n", p.ID, p.Name, p.Quantity, p.Price)
}
*/