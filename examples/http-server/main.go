package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/voseghale/batching"
)

const maxBatchSize = 1000

func main() {
	// Create orchestrator
	orch := relayer.New(
		relayer.WithTimeout(10 * time.Second),
		relayer.WithMaxConcurrency(100),       // Limit concurrent recipe executions
		relayer.WithMaxBatchSize(maxBatchSize), // Use same limit as HTTP validation
	)

	// Register sample recipes
	setupRecipes(orch)

	// Create HTTP server
	mux := http.NewServeMux()

	// Batch endpoint
	mux.HandleFunc("/batch", func(w http.ResponseWriter, r *http.Request) {
		handleBatch(w, r, orch)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
			log.Printf("Error encoding health response: %v", err)
		}
	})

	// Start server with explicit timeouts
	addr := ":8080"
	server := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    15 * time.Second,  // Prevent slow read attacks
		WriteTimeout:   15 * time.Second,  // Prevent slow write attacks
		IdleTimeout:    60 * time.Second,  // Connection reuse timeout
		MaxHeaderBytes: 1 << 20,           // 1 MB max header size
	}

	log.Printf("Starting HTTP server on %s", addr)
	log.Printf("Try: curl -X POST http://localhost:8080/batch -H 'Content-Type: application/json' -d @sample.json")
	log.Printf("Sample payload in sample.json (you can create this file):")
	printSampleJSON()

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleBatch(w http.ResponseWriter, r *http.Request, orch *relayer.Orchestrator) {
	// SECURITY NOTE: This is an example server for demonstration purposes.
	// In production, you should add:
	// - Authentication (API keys, JWT tokens, OAuth)
	// - Rate limiting (per-IP or per-tenant)
	// - Request validation (schema validation, input sanitization)
	// - TLS/HTTPS (using server.ListenAndServeTLS)
	// - Request ID tracking for observability
	// - CORS configuration for web clients

	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to prevent memory exhaustion
	const maxBodySize = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	// Parse request body
	var batch []relayer.SubRequest
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		log.Printf("JSON decode error: %v", err) // Log details internally only
		return
	}

	// Validate batch
	if len(batch) == 0 {
		http.Error(w, "Empty batch", http.StatusBadRequest)
		return
	}

	if len(batch) > maxBatchSize {
		http.Error(w, fmt.Sprintf("Batch too large (max %d)", maxBatchSize), http.StatusBadRequest)
		return
	}

	// Execute batch
	ctx := r.Context()
	results := orch.ExecuteBatch(ctx, batch)

	// Return results
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"summary": map[string]interface{}{
			"total":     len(results),
			"successes": len(relayer.FilterSuccess(results)),
			"failures":  len(results) - len(relayer.FilterSuccess(results)),
		},
	}); err != nil {
		log.Printf("Error encoding response: %v", err)
	}

	log.Printf("Processed batch: %d requests, %d successes",
		len(results), len(relayer.FilterSuccess(results)))
}

func setupRecipes(orch *relayer.Orchestrator) {
	// Echo recipe - returns payload as-is
	orch.RegisterRecipe("echo", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	// Uppercase recipe - converts string to uppercase
	orch.RegisterRecipe("uppercase", func(ctx context.Context, payload interface{}) (interface{}, error) {
		str, ok := payload.(string)
		if !ok {
			return nil, fmt.Errorf("payload must be string")
		}
		return fmt.Sprintf("%s (uppercase: %s)", str, fmt.Sprint(str)), nil
	})

	// Math recipe - performs arithmetic
	orch.RegisterRecipe("math", func(ctx context.Context, payload interface{}) (interface{}, error) {
		data, ok := payload.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("payload must be object")
		}

		op, _ := data["op"].(string)
		a, ok1 := data["a"].(float64)
		b, ok2 := data["b"].(float64)

		if !ok1 || !ok2 {
			return nil, fmt.Errorf("a and b must be numbers")
		}

		var result float64
		switch op {
		case "add":
			result = a + b
		case "subtract":
			result = a - b
		case "multiply":
			result = a * b
		case "divide":
			if b == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			result = a / b
		default:
			return nil, fmt.Errorf("unknown operation: %s", op)
		}

		return result, nil
	})

	// User info recipe - simulates database lookup
	orch.RegisterRecipe("user-info", func(ctx context.Context, payload interface{}) (interface{}, error) {
		tenantID, _ := relayer.TenantID(ctx)
		userID, ok := payload.(string)
		if !ok {
			return nil, fmt.Errorf("payload must be user ID string")
		}

		// Simulate database query
		time.Sleep(10 * time.Millisecond)

		return map[string]interface{}{
			"user_id":   userID,
			"tenant_id": tenantID,
			"name":      fmt.Sprintf("User %s", userID),
			"email":     fmt.Sprintf("%s@%s.com", userID, tenantID),
		}, nil
	})

	log.Println("Registered recipes: echo, uppercase, math, user-info")
}

func printSampleJSON() {
	sample := `
[
  {
    "id": "1",
    "tenant_id": "company-a",
    "recipe": "echo",
    "payload": "Hello World"
  },
  {
    "id": "2",
    "tenant_id": "company-a",
    "recipe": "uppercase",
    "payload": "hello"
  },
  {
    "id": "3",
    "tenant_id": "company-b",
    "recipe": "math",
    "payload": {"op": "add", "a": 10, "b": 32}
  },
  {
    "id": "4",
    "tenant_id": "company-b",
    "recipe": "user-info",
    "payload": "user123"
  }
]
`
	fmt.Println(sample)
}
