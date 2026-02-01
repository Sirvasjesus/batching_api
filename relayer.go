package relayer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// panicError is a sentinel error type to distinguish panics from regular errors
type panicError struct{}

func (e *panicError) Error() string {
	return "internal error during recipe execution"
}

// Orchestrator manages recipe registration and batch execution.
// It provides concurrent request processing with tenant isolation,
// panic recovery, and observability hooks.
type Orchestrator struct {
	registry       map[string]Handler
	recipeOptions  map[string]*RecipeOption
	mu             sync.RWMutex
	timeout        time.Duration
	executionHook  ExecutionHook
	panicHook      PanicHook
	maxConcurrency int
	maxBatchSize   int           // Maximum batch size (0 = unlimited)
	semaphore      chan struct{} // For concurrency limiting
}

// New creates a new Orchestrator with the provided options.
// If no options are specified, sensible defaults are used:
//   - timeout: 5 seconds
//   - maxConcurrency: unlimited (0)
//   - hooks: no-op implementations
//
// Example:
//
//	orch := relayer.New(
//		relayer.WithTimeout(10 * time.Second),
//		relayer.WithMaxConcurrency(100),
//		relayer.WithExecutionHook(&MyLoggingHook{}),
//	)
func New(opts ...Option) *Orchestrator {
	o := &Orchestrator{
		registry:       make(map[string]Handler),
		recipeOptions:  make(map[string]*RecipeOption),
		timeout:        5 * time.Second, // Default timeout
		executionHook:  &NoOpHook{},
		panicHook:      &NoOpHook{},
		maxConcurrency: 0, // Unlimited by default
	}

	for _, opt := range opts {
		opt(o)
	}

	// Initialize semaphore if concurrency limiting is enabled
	if o.maxConcurrency > 0 {
		o.semaphore = make(chan struct{}, o.maxConcurrency)
	}

	return o
}

// RegisterRecipe registers a handler function for a recipe name.
// The recipe name must be unique. If a recipe with the same name
// already exists, it will be replaced.
//
// Optional RecipeOption can be provided to override default settings
// for this specific recipe (e.g., custom timeout).
//
// Example:
//
//	orch.RegisterRecipe("get-user", func(ctx context.Context, payload interface{}) (interface{}, error) {
//		tenantID, _ := relayer.TenantID(ctx)
//		userID := payload.(string)
//		// Fetch user for tenant
//		return user, nil
//	})
//
//	// With custom timeout
//	orch.RegisterRecipe("slow-operation", handler, &relayer.RecipeOption{
//		Timeout: 30 * time.Second,
//	})
func RegisterRecipe(o *Orchestrator, name string, handler Handler, opts ...*RecipeOption) {
	// Validate inputs
	if name == "" {
		panic("recipe name cannot be empty")
	}
	if handler == nil {
		panic("recipe handler cannot be nil")
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.registry[name] = handler
	if len(opts) > 0 && opts[0] != nil {
		o.recipeOptions[name] = opts[0]
	}
}

// RegisterRecipe registers a handler function for a recipe name.
// See package-level RegisterRecipe for details.
func (o *Orchestrator) RegisterRecipe(name string, handler Handler, opts ...*RecipeOption) {
	RegisterRecipe(o, name, handler, opts...)
}

// ExecuteBatch processes a batch of requests concurrently.
// Each request is executed in its own goroutine with tenant isolation.
// Returns responses in the same order as the input batch.
//
// The context can be used for cancellation of the entire batch.
// Individual requests also have their own timeout contexts.
//
// Example:
//
//	batch := []relayer.SubRequest{
//		{ID: "1", TenantID: "tenant-a", Recipe: "get-user", Payload: "user-123"},
//		{ID: "2", TenantID: "tenant-b", Recipe: "get-user", Payload: "user-456"},
//	}
//	results := orch.ExecuteBatch(ctx, batch)
//	successes := relayer.FilterSuccess(results)
func (o *Orchestrator) ExecuteBatch(ctx context.Context, batch []SubRequest) []Response {
	// Check batch size limit
	if o.maxBatchSize > 0 && len(batch) > o.maxBatchSize {
		// Return error response for all requests in oversized batch
		results := make([]Response, len(batch))
		for i, req := range batch {
			results[i] = Response{
				ID:       req.ID,
				Status:   413, // HTTP 413 Payload Too Large
				TenantID: req.TenantID,
				Error: &Error{
					Code:    ErrCodeBatchTooLarge,
					Message: fmt.Sprintf("batch size %d exceeds limit of %d", len(batch), o.maxBatchSize),
				},
			}
		}
		return results
	}

	results := make([]Response, len(batch))
	var wg sync.WaitGroup

	for i, req := range batch {
		wg.Add(1)
		go o.executeRequest(ctx, &wg, req, &results[i])
	}

	wg.Wait()
	return results
}

// executeRequest processes a single request in a goroutine.
// It handles concurrency limiting, context enrichment, timeout, and hooks.
func (o *Orchestrator) executeRequest(ctx context.Context, wg *sync.WaitGroup, req SubRequest, result *Response) {
	defer wg.Done()

	// Acquire semaphore if concurrency limiting is enabled
	if o.maxConcurrency > 0 {
		o.semaphore <- struct{}{}
		defer func() { <-o.semaphore }()
	}

	start := time.Now()

	// Validate request fields
	if req.ID == "" || req.TenantID == "" || req.Recipe == "" {
		*result = Response{
			ID:       req.ID,
			Status:   400,
			TenantID: req.TenantID,
			Duration: time.Since(start),
			Error: &Error{
				Code:    ErrCodeInvalidRequest,
				Message: "request must have non-empty ID, TenantID, and Recipe",
			},
		}
		return
	}

	// Enrich context with request metadata
	taskCtx := WithTenantID(ctx, req.TenantID)
	taskCtx = WithRequestID(taskCtx, req.ID)
	taskCtx = WithRecipeName(taskCtx, req.Recipe)

	// Get recipe timeout (check for per-recipe override)
	timeout := o.timeout
	o.mu.RLock()
	if recipeOpt, exists := o.recipeOptions[req.Recipe]; exists && recipeOpt.Timeout > 0 {
		timeout = recipeOpt.Timeout
	}
	o.mu.RUnlock()

	// Apply timeout
	taskCtx, cancel := context.WithTimeout(taskCtx, timeout)
	defer cancel()

	// Execute with hooks and panic recovery
	o.executionHook.OnStart(taskCtx, req)

	resp := o.safeExecute(taskCtx, req)
	resp.Duration = time.Since(start)
	resp.TenantID = req.TenantID

	o.executionHook.OnComplete(taskCtx, req, resp, resp.Duration)

	*result = resp
}

// safeExecute executes the recipe with panic recovery.
// Returns a Response with appropriate status code and error information.
func (o *Orchestrator) safeExecute(ctx context.Context, req SubRequest) Response {
	// Look up handler
	o.mu.RLock()
	handler, exists := o.registry[req.Recipe]
	o.mu.RUnlock()

	if !exists {
		return Response{
			ID:     req.ID,
			Status: 404,
			Error: &Error{
				Code:    ErrCodeRecipeNotFound,
				Message: fmt.Sprintf("recipe '%s' not found", req.Recipe),
			},
		}
	}

	// Execute handler with panic recovery
	var data interface{}
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Call panic hook with full panic value for internal logging/alerting
				// The hook can log the panic value and stack trace internally
				o.panicHook.OnPanic(ctx, req, r)
				// Set sentinel error (no sensitive information in message)
				err = &panicError{}
			}
		}()
		data, err = handler(ctx, req.Payload)
	}()

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		return Response{
			ID:     req.ID,
			Status: 504,
			Error: &Error{
				Code:    ErrCodeTimeout,
				Message: "recipe execution timed out",
			},
		}
	}

	// Handle execution error
	if err != nil {
		// Check if error is from a panic
		if _, isPanic := err.(*panicError); isPanic {
			return Response{
				ID:     req.ID,
				Status: 500,
				Error: &Error{
					Code:    ErrCodePanic,
					Message: err.Error(), // Generic message from panicError
				},
			}
		}

		// Regular recipe error
		return Response{
			ID:     req.ID,
			Status: 500,
			Error: &Error{
				Code:    ErrCodeRecipeExecution,
				Message: err.Error(),
			},
		}
	}

	return Response{
		ID:     req.ID,
		Status: 200,
		Data:   data,
	}
}
