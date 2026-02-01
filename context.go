package relayer

import "context"

// contextKey is an unexported type for context keys to prevent collisions.
// Using a custom type ensures that our context keys don't collide with
// keys from other packages.
type contextKey int

const (
	tenantIDKey contextKey = iota
	requestIDKey
	recipeNameKey
)

// WithTenantID returns a new context with the tenant ID embedded.
// This is used internally by the orchestrator to inject tenant isolation
// into recipe contexts.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantID extracts the tenant ID from the context.
// Returns the tenant ID and true if present, or empty string and false if not.
//
// Example usage in a recipe:
//
//	func MyRecipe(ctx context.Context, payload interface{}) (interface{}, error) {
//		tenantID, ok := relayer.TenantID(ctx)
//		if !ok {
//			return nil, errors.New("tenant ID not found in context")
//		}
//		// Use tenantID for scoped database queries
//		return result, nil
//	}
func TenantID(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(tenantIDKey).(string)
	return tenantID, ok
}

// WithRequestID returns a new context with the request ID embedded.
// This is used internally by the orchestrator to track individual requests
// through their lifecycle.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestID extracts the request ID from the context.
// Returns the request ID and true if present, or empty string and false if not.
//
// Useful for logging and tracing:
//
//	func MyRecipe(ctx context.Context, payload interface{}) (interface{}, error) {
//		requestID, _ := relayer.RequestID(ctx)
//		log.Printf("Processing request %s", requestID)
//		return result, nil
//	}
func RequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	return requestID, ok
}

// WithRecipeName returns a new context with the recipe name embedded.
// This is used internally by the orchestrator to track which recipe
// is being executed.
func WithRecipeName(ctx context.Context, recipeName string) context.Context {
	return context.WithValue(ctx, recipeNameKey, recipeName)
}

// RecipeName extracts the recipe name from the context.
// Returns the recipe name and true if present, or empty string and false if not.
//
// Useful for middleware and hooks:
//
//	func (h *LoggingHook) OnStart(ctx context.Context, req SubRequest) {
//		recipeName, _ := relayer.RecipeName(ctx)
//		log.Printf("Starting recipe: %s", recipeName)
//	}
func RecipeName(ctx context.Context) (string, bool) {
	recipeName, ok := ctx.Value(recipeNameKey).(string)
	return recipeName, ok
}
