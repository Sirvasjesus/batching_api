package relayer

import (
	"context"
	"fmt"
	"time"
)

// SubRequest represents a single request in a batch.
// It contains all necessary information to identify and process a request
// for a specific tenant using a named recipe.
type SubRequest struct {
	ID       string      `json:"id"`        // Unique request identifier
	TenantID string      `json:"tenant_id"` // Tenant identifier for isolation
	Recipe   string      `json:"recipe"`    // Name of the recipe to execute
	Payload  interface{} `json:"payload"`   // Request payload (any JSON-serializable type)
}

// Response represents the result of processing a SubRequest.
// It includes the request ID, status code, data, error information,
// execution duration, and tenant ID.
type Response struct {
	ID       string        `json:"id"`                 // Request ID matching SubRequest.ID
	Status   int           `json:"status"`             // HTTP-style status code (200, 404, 500, etc.)
	Data     interface{}   `json:"data,omitempty"`     // Response data from successful execution
	Error    *Error        `json:"error,omitempty"`    // Error details if execution failed
	Duration time.Duration `json:"duration_ms"`        // Execution duration in milliseconds
	TenantID string        `json:"tenant_id,omitempty"` // Tenant ID for filtering
}

// Error provides structured error information with code, message, and optional details.
type Error struct {
	Code    string                 `json:"code"`              // Error code (e.g., RECIPE_NOT_FOUND)
	Message string                 `json:"message"`           // Human-readable error message
	Details map[string]interface{} `json:"details,omitempty"` // Additional error context
}

// Error implements the error interface for Error type.
func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Common error codes used throughout the library
const (
	ErrCodeRecipeNotFound  = "RECIPE_NOT_FOUND"  // Recipe name not registered
	ErrCodeTimeout         = "TIMEOUT"           // Recipe execution timeout
	ErrCodePanic           = "PANIC"             // Recipe panicked during execution
	ErrCodeRecipeExecution = "RECIPE_EXECUTION"  // Recipe returned error
	ErrCodeInvalidPayload  = "INVALID_PAYLOAD"   // Payload validation failed
	ErrCodeBatchTooLarge   = "BATCH_TOO_LARGE"   // Batch size exceeds limit
	ErrCodeInvalidRequest  = "INVALID_REQUEST"   // Request validation failed
)

// Handler is the function signature for recipe implementations.
// Recipes receive a context (with tenant metadata) and payload,
// and return data or an error.
//
// Example:
//
//	func MyRecipe(ctx context.Context, payload interface{}) (interface{}, error) {
//		tenantID, _ := relayer.TenantID(ctx)
//		// Process request for tenant
//		return result, nil
//	}
type Handler func(ctx context.Context, payload interface{}) (interface{}, error)

// FilterSuccess returns only successful responses (2xx status codes).
// This implements the "partial success" pattern where only successful
// results are returned to the caller.
//
// Example:
//
//	responses := orchestrator.ExecuteBatch(ctx, batch)
//	successes := relayer.FilterSuccess(responses)
func FilterSuccess(responses []Response) []Response {
	successes := make([]Response, 0, len(responses))
	for _, resp := range responses {
		if resp.Status >= 200 && resp.Status < 300 {
			successes = append(successes, resp)
		}
	}
	return successes
}

// FilterByStatus returns responses matching a specific status code.
// Useful for processing errors of a specific type.
//
// Example:
//
//	notFound := relayer.FilterByStatus(responses, 404)
func FilterByStatus(responses []Response, status int) []Response {
	filtered := make([]Response, 0, len(responses))
	for _, resp := range responses {
		if resp.Status == status {
			filtered = append(filtered, resp)
		}
	}
	return filtered
}

// FilterByTenant returns responses for a specific tenant.
// Useful for tenant-specific result processing.
//
// Example:
//
//	tenantResults := relayer.FilterByTenant(responses, "tenant-123")
func FilterByTenant(responses []Response, tenantID string) []Response {
	filtered := make([]Response, 0, len(responses))
	for _, resp := range responses {
		if resp.TenantID == tenantID {
			filtered = append(filtered, resp)
		}
	}
	return filtered
}
