package relayer

import "time"

// Option configures an Orchestrator instance.
// Options are applied when creating a new Orchestrator via New().
type Option func(*Orchestrator)

// WithTimeout sets the default timeout for recipe execution.
// Individual recipes can override this with RecipeOption.
// Panics if timeout is <= 0.
//
// Example:
//
//	orch := relayer.New(relayer.WithTimeout(5 * time.Second))
func WithTimeout(timeout time.Duration) Option {
	return func(o *Orchestrator) {
		if timeout <= 0 {
			panic("timeout must be positive")
		}
		o.timeout = timeout
	}
}

// WithExecutionHook sets the execution hook for lifecycle events.
// The hook is called before and after each recipe execution.
// If nil is provided, NoOpHook is used as a safe default.
//
// Example:
//
//	orch := relayer.New(relayer.WithExecutionHook(&MyLoggingHook{}))
func WithExecutionHook(hook ExecutionHook) Option {
	return func(o *Orchestrator) {
		if hook == nil {
			o.executionHook = &NoOpHook{}
		} else {
			o.executionHook = hook
		}
	}
}

// WithPanicHook sets the panic recovery hook.
// The hook is called when a recipe panics during execution.
// If nil is provided, NoOpHook is used as a safe default.
//
// Example:
//
//	orch := relayer.New(relayer.WithPanicHook(&MyAlertingHook{}))
func WithPanicHook(hook PanicHook) Option {
	return func(o *Orchestrator) {
		if hook == nil {
			o.panicHook = &NoOpHook{}
		} else {
			o.panicHook = hook
		}
	}
}

// WithMaxConcurrency limits the number of concurrent recipe executions.
// Set to 0 for unlimited concurrency (default).
// Panics if max is < 0.
// Useful for controlling resource usage and back-pressure.
//
// Example:
//
//	orch := relayer.New(relayer.WithMaxConcurrency(100))
func WithMaxConcurrency(max int) Option {
	return func(o *Orchestrator) {
		if max < 0 {
			panic("max concurrency must be non-negative")
		}
		o.maxConcurrency = max
	}
}

// WithMaxBatchSize limits the maximum number of requests in a batch.
// Set to 0 for unlimited batch size (default, not recommended for production).
// Panics if max is < 0.
// Prevents resource exhaustion from oversized batches.
//
// Recommended values: 100-10000 depending on system resources.
//
// Example:
//
//	orch := relayer.New(relayer.WithMaxBatchSize(1000))
func WithMaxBatchSize(max int) Option {
	return func(o *Orchestrator) {
		if max < 0 {
			panic("max batch size must be non-negative")
		}
		o.maxBatchSize = max
	}
}

// RecipeOption configures a specific recipe.
// Allows per-recipe timeout overrides and other recipe-specific settings.
type RecipeOption struct {
	Timeout time.Duration // Override default timeout for this recipe
}
