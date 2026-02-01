package relayer

import "time"

// Option configures an Orchestrator instance.
// Options are applied when creating a new Orchestrator via New().
type Option func(*Orchestrator)

// WithTimeout sets the default timeout for recipe execution.
// Individual recipes can override this with RecipeOption.
//
// Example:
//
//	orch := relayer.New(relayer.WithTimeout(5 * time.Second))
func WithTimeout(timeout time.Duration) Option {
	return func(o *Orchestrator) {
		o.timeout = timeout
	}
}

// WithExecutionHook sets the execution hook for lifecycle events.
// The hook is called before and after each recipe execution.
//
// Example:
//
//	orch := relayer.New(relayer.WithExecutionHook(&MyLoggingHook{}))
func WithExecutionHook(hook ExecutionHook) Option {
	return func(o *Orchestrator) {
		o.executionHook = hook
	}
}

// WithPanicHook sets the panic recovery hook.
// The hook is called when a recipe panics during execution.
//
// Example:
//
//	orch := relayer.New(relayer.WithPanicHook(&MyAlertingHook{}))
func WithPanicHook(hook PanicHook) Option {
	return func(o *Orchestrator) {
		o.panicHook = hook
	}
}

// WithMaxConcurrency limits the number of concurrent recipe executions.
// Set to 0 for unlimited concurrency (default).
// Useful for controlling resource usage and back-pressure.
//
// Example:
//
//	orch := relayer.New(relayer.WithMaxConcurrency(100))
func WithMaxConcurrency(max int) Option {
	return func(o *Orchestrator) {
		o.maxConcurrency = max
	}
}

// RecipeOption configures a specific recipe.
// Allows per-recipe timeout overrides and other recipe-specific settings.
type RecipeOption struct {
	Timeout time.Duration // Override default timeout for this recipe
}
