package relayer

import (
	"context"
	"time"
)

// ExecutionHook provides callbacks for recipe execution lifecycle events.
// Implementations can use these hooks for logging, metrics, tracing, etc.
//
// Example implementation:
//
//	type LoggingHook struct{}
//
//	func (h *LoggingHook) OnStart(ctx context.Context, req SubRequest) {
//		log.Printf("Starting: tenant=%s recipe=%s id=%s",
//			req.TenantID, req.Recipe, req.ID)
//	}
//
//	func (h *LoggingHook) OnComplete(ctx context.Context, req SubRequest,
//		resp Response, duration time.Duration) {
//		log.Printf("Completed: id=%s status=%d duration=%v",
//			resp.ID, resp.Status, duration)
//	}
type ExecutionHook interface {
	// OnStart is called before recipe execution begins.
	// The context contains tenant ID, request ID, and recipe name.
	OnStart(ctx context.Context, req SubRequest)

	// OnComplete is called after recipe execution finishes.
	// Receives the request, response, and execution duration.
	// Called for both successful and failed executions.
	OnComplete(ctx context.Context, req SubRequest, resp Response, duration time.Duration)
}

// PanicHook provides a callback when a recipe panics during execution.
// Implementations can use this for alerting, error reporting, etc.
//
// Example implementation:
//
//	type AlertingHook struct{}
//
//	func (h *AlertingHook) OnPanic(ctx context.Context, req SubRequest, recovered interface{}) {
//		tenantID, _ := relayer.TenantID(ctx)
//		alert.Send("Recipe panic: tenant=%s recipe=%s error=%v",
//			tenantID, req.Recipe, recovered)
//	}
type PanicHook interface {
	// OnPanic is called when a recipe panics.
	// The recovered value is the panic value (interface{}).
	// The context contains tenant ID, request ID, and recipe name.
	OnPanic(ctx context.Context, req SubRequest, recovered interface{})
}

// NoOpHook provides default no-op implementations of all hook interfaces.
// Useful as a base for partial hook implementations or as a default.
type NoOpHook struct{}

// OnStart is a no-op implementation.
func (h *NoOpHook) OnStart(ctx context.Context, req SubRequest) {}

// OnComplete is a no-op implementation.
func (h *NoOpHook) OnComplete(ctx context.Context, req SubRequest, resp Response, duration time.Duration) {
}

// OnPanic is a no-op implementation.
func (h *NoOpHook) OnPanic(ctx context.Context, req SubRequest, recovered interface{}) {}
