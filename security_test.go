package relayer

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// Security Test Suite for Phase 1 Fixes

// R-1: Test that panic errors don't leak stack traces
func TestSecurity_PanicNoStackTrace(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("panic-recipe", func(ctx context.Context, payload interface{}) (interface{}, error) {
		panic("sensitive internal error with file paths")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "panic-recipe"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]

	// Verify error code is PANIC, not RECIPE_EXECUTION
	if result.Error.Code != ErrCodePanic {
		t.Errorf("Error code = %s, want %s", result.Error.Code, ErrCodePanic)
	}

	// Verify error message is generic
	if result.Error.Message != "internal error during recipe execution" {
		t.Errorf("Error message = %q, want generic message", result.Error.Message)
	}

	// Verify NO stack trace information in message
	errorMsg := result.Error.Message
	forbiddenStrings := []string{
		"goroutine",
		"panic:",
		".go:",
		"runtime/",
		"/home/",
		"Stack:",
		"\n", // No multi-line stack traces
	}

	for _, forbidden := range forbiddenStrings {
		if strings.Contains(errorMsg, forbidden) {
			t.Errorf("Error message contains forbidden string %q: %s", forbidden, errorMsg)
		}
	}
}

// R-1: Test that panic hook still receives full information
func TestSecurity_PanicHookReceivesFullInfo(t *testing.T) {
	panicHook := &mockPanicHook{}
	orch := New(WithPanicHook(panicHook))

	orch.RegisterRecipe("panic-recipe", func(ctx context.Context, payload interface{}) (interface{}, error) {
		panic("test panic value")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "panic-recipe"},
	}

	orch.ExecuteBatch(context.Background(), batch)

	panicCalls := panicHook.getPanicCalls()
	if len(panicCalls) != 1 {
		t.Fatalf("Expected 1 panic hook call, got %d", len(panicCalls))
	}

	// Verify hook received the actual panic value
	if panicCalls[0].recovered != "test panic value" {
		t.Errorf("Panic hook recovered = %v, want 'test panic value'", panicCalls[0].recovered)
	}
}

// R-2: Test batch size limit enforcement
func TestSecurity_BatchSizeLimit(t *testing.T) {
	orch := New(WithMaxBatchSize(10))

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	})

	// Create batch larger than limit
	batch := make([]SubRequest, 20)
	for i := 0; i < 20; i++ {
		batch[i] = SubRequest{
			ID:       string(rune('a' + i)),
			TenantID: "tenant-a",
			Recipe:   "test",
		}
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	// All requests should fail with BATCH_TOO_LARGE
	if len(results) != 20 {
		t.Fatalf("Expected 20 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Status != 413 {
			t.Errorf("Result %d status = %d, want 413", i, result.Status)
		}

		if result.Error.Code != ErrCodeBatchTooLarge {
			t.Errorf("Result %d error code = %s, want %s", i, result.Error.Code, ErrCodeBatchTooLarge)
		}

		expectedMsg := "batch size 20 exceeds limit of 10"
		if result.Error.Message != expectedMsg {
			t.Errorf("Result %d error message = %q, want %q", i, result.Error.Message, expectedMsg)
		}
	}
}

// R-2: Test unlimited batch size (default)
func TestSecurity_UnlimitedBatchSize(t *testing.T) {
	orch := New() // maxBatchSize defaults to 0 (unlimited)

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	})

	// Create large batch
	batch := make([]SubRequest, 100)
	for i := 0; i < 100; i++ {
		batch[i] = SubRequest{
			ID:       string(rune(i)),
			TenantID: "tenant-a",
			Recipe:   "test",
		}
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	// All should succeed
	if len(results) != 100 {
		t.Fatalf("Expected 100 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Status != 200 {
			t.Errorf("Result %d status = %d, want 200", i, result.Status)
		}
	}
}

// R-3: Test empty tenant ID validation
func TestSecurity_EmptyTenantIDRejected(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		tenantID, _ := TenantID(ctx)
		if tenantID == "" {
			t.Error("Recipe received empty tenant ID!")
		}
		return "ok", nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "", Recipe: "test"}, // Empty tenant ID
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 400 {
		t.Errorf("Status = %d, want 400 for empty tenant ID", results[0].Status)
	}

	if results[0].Error.Code != ErrCodeInvalidRequest {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodeInvalidRequest)
	}
}

// R-3: Test empty request ID validation
func TestSecurity_EmptyRequestIDRejected(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	})

	batch := []SubRequest{
		{ID: "", TenantID: "tenant-a", Recipe: "test"}, // Empty ID
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 400 {
		t.Errorf("Status = %d, want 400 for empty request ID", results[0].Status)
	}

	if results[0].Error.Code != ErrCodeInvalidRequest {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodeInvalidRequest)
	}
}

// R-3: Test empty recipe name validation
func TestSecurity_EmptyRecipeRejected(t *testing.T) {
	orch := New()

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: ""}, // Empty recipe
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 400 {
		t.Errorf("Status = %d, want 400 for empty recipe", results[0].Status)
	}

	if results[0].Error.Code != ErrCodeInvalidRequest {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodeInvalidRequest)
	}
}

// R-4: Test nil handler registration panics
func TestSecurity_NilHandlerPanics(t *testing.T) {
	orch := New()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when registering nil handler, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "handler cannot be nil") {
				t.Errorf("Panic message = %q, want 'handler cannot be nil'", msg)
			}
		}
	}()

	orch.RegisterRecipe("test", nil)
}

// R-4: Test empty recipe name registration panics
func TestSecurity_EmptyRecipeNamePanics(t *testing.T) {
	orch := New()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when registering empty recipe name, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "recipe name cannot be empty") {
				t.Errorf("Panic message = %q, want 'recipe name cannot be empty'", msg)
			}
		}
	}()

	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	}

	orch.RegisterRecipe("", handler)
}

// R-5: Test nil execution hook defaults to NoOpHook
func TestSecurity_NilExecutionHookSafe(t *testing.T) {
	orch := New(WithExecutionHook(nil))

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "test"},
	}

	// Should not panic
	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 200 {
		t.Errorf("Status = %d, want 200 (nil hook should be safe)", results[0].Status)
	}
}

// R-5: Test nil panic hook defaults to NoOpHook
func TestSecurity_NilPanicHookSafe(t *testing.T) {
	orch := New(WithPanicHook(nil))

	orch.RegisterRecipe("panic-recipe", func(ctx context.Context, payload interface{}) (interface{}, error) {
		panic("test")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "panic-recipe"},
	}

	// Should not panic even though recipe panics and panic hook is nil
	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 500 {
		t.Errorf("Status = %d, want 500", results[0].Status)
	}

	if results[0].Error.Code != ErrCodePanic {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodePanic)
	}
}

// Integration: Test tenant isolation with validation
func TestSecurity_TenantIsolationWithValidation(t *testing.T) {
	orch := New()

	var receivedTenants []string
	var mu sync.Mutex

	orch.RegisterRecipe("capture", func(ctx context.Context, payload interface{}) (interface{}, error) {
		tenantID, ok := TenantID(ctx)
		if !ok {
			return nil, errors.New("no tenant ID in context")
		}
		mu.Lock()
		receivedTenants = append(receivedTenants, tenantID)
		mu.Unlock()
		return tenantID, nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "capture"},
		{ID: "2", TenantID: "", Recipe: "capture"}, // Invalid - empty tenant
		{ID: "3", TenantID: "tenant-b", Recipe: "capture"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	// Request 1: Success
	if results[0].Status != 200 {
		t.Errorf("Result 0 status = %d, want 200", results[0].Status)
	}

	// Request 2: Rejected due to empty tenant ID
	if results[1].Status != 400 {
		t.Errorf("Result 1 status = %d, want 400 (empty tenant)", results[1].Status)
	}

	// Request 3: Success
	if results[2].Status != 200 {
		t.Errorf("Result 2 status = %d, want 200", results[2].Status)
	}

	// Only valid tenants should have been processed
	mu.Lock()
	receivedCount := len(receivedTenants)
	tenantsCopy := make([]string, len(receivedTenants))
	copy(tenantsCopy, receivedTenants)
	mu.Unlock()

	if receivedCount != 2 {
		t.Errorf("Received %d tenants, want 2 (invalid request not processed)", receivedCount)
	}

	// Check that both valid tenants were processed (order doesn't matter due to concurrency)
	tenantMap := make(map[string]bool)
	for _, tenant := range tenantsCopy {
		tenantMap[tenant] = true
	}

	if !tenantMap["tenant-a"] || !tenantMap["tenant-b"] {
		t.Errorf("Received tenants = %v, want both tenant-a and tenant-b", tenantsCopy)
	}
}

// Regression: Ensure fixes don't break existing functionality
func TestSecurity_RegressionCheck(t *testing.T) {
	orch := New(
		WithTimeout(1*time.Second),
		WithMaxBatchSize(100),
		WithMaxConcurrency(10),
	)

	orch.RegisterRecipe("echo", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	orch.RegisterRecipe("error", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, errors.New("test error")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "echo", Payload: "test"},
		{ID: "2", TenantID: "tenant-b", Recipe: "error"},
		{ID: "3", TenantID: "tenant-c", Recipe: "nonexistent"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	// Result 1: Success
	if results[0].Status != 200 {
		t.Errorf("Result 0 status = %d, want 200", results[0].Status)
	}

	// Result 2: Recipe error
	if results[1].Status != 500 || results[1].Error.Code != ErrCodeRecipeExecution {
		t.Errorf("Result 1 status = %d code = %s, want 500 and RECIPE_EXECUTION",
			results[1].Status, results[1].Error.Code)
	}

	// Result 3: Recipe not found
	if results[2].Status != 404 || results[2].Error.Code != ErrCodeRecipeNotFound {
		t.Errorf("Result 2 status = %d code = %s, want 404 and RECIPE_NOT_FOUND",
			results[2].Status, results[2].Error.Code)
	}
}

// Phase 2 Security Tests

// R-6: Test semaphore respects context cancellation
func TestSecurity_SemaphoreRespectsContextCancellation(t *testing.T) {
	// Create orchestrator with very low concurrency limit
	orch := New(WithMaxConcurrency(1))

	// Register a slow recipe that blocks
	blockChan := make(chan struct{})
	orch.RegisterRecipe("slow", func(ctx context.Context, payload interface{}) (interface{}, error) {
		<-blockChan // Block until we release
		return "ok", nil
	})

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start first request to occupy the semaphore slot
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		batch := []SubRequest{
			{ID: "1", TenantID: "tenant-a", Recipe: "slow"},
		}
		orch.ExecuteBatch(context.Background(), batch)
	}()

	// Give first request time to acquire semaphore
	time.Sleep(50 * time.Millisecond)

	// Now submit second request that should wait on semaphore
	results := make(chan []Response, 1)
	go func() {
		batch := []SubRequest{
			{ID: "2", TenantID: "tenant-b", Recipe: "slow"},
		}
		results <- orch.ExecuteBatch(ctx, batch)
	}()

	// Give second request time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Cancel the context while second request is waiting
	cancel()

	// Get result from second request
	result := <-results

	// Verify second request got cancellation error
	if result[0].Status != 504 {
		t.Errorf("Status = %d, want 504 for cancelled request", result[0].Status)
	}

	if result[0].Error.Code != ErrCodeTimeout {
		t.Errorf("Error code = %s, want %s", result[0].Error.Code, ErrCodeTimeout)
	}

	if !strings.Contains(result[0].Error.Message, "cancelled while waiting") {
		t.Errorf("Error message = %q, want message about cancellation", result[0].Error.Message)
	}

	// Release first request and wait for completion
	close(blockChan)
	wg.Wait()
}

// R-7: Test RegisterRecipeStrict rejects duplicates
func TestSecurity_RegisterRecipeStrict_RejectsDuplicates(t *testing.T) {
	orch := New()

	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	}

	// First registration should succeed
	err := orch.RegisterRecipeStrict("test", handler)
	if err != nil {
		t.Errorf("First registration failed: %v", err)
	}

	// Second registration should fail
	err = orch.RegisterRecipeStrict("test", handler)
	if err == nil {
		t.Error("Expected error for duplicate registration, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "already registered") {
		t.Errorf("Error message = %q, want 'already registered'", err.Error())
	}
}

// R-7: Test RegisterRecipeStrict with empty name panics
func TestSecurity_RegisterRecipeStrict_EmptyNamePanics(t *testing.T) {
	orch := New()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for empty recipe name, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "recipe name cannot be empty") {
				t.Errorf("Panic message = %q, want 'recipe name cannot be empty'", msg)
			}
		}
	}()

	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	}

	_ = orch.RegisterRecipeStrict("", handler)
}

// R-7: Test RegisterRecipeStrict with nil handler panics
func TestSecurity_RegisterRecipeStrict_NilHandlerPanics(t *testing.T) {
	orch := New()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil handler, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "handler cannot be nil") {
				t.Errorf("Panic message = %q, want 'handler cannot be nil'", msg)
			}
		}
	}()

	_ = orch.RegisterRecipeStrict("test", nil)
}

// R-8: Test WithTimeout panics on zero timeout
func TestSecurity_WithTimeout_ZeroPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for zero timeout, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "timeout must be positive") {
				t.Errorf("Panic message = %q, want 'timeout must be positive'", msg)
			}
		}
	}()

	_ = New(WithTimeout(0))
}

// R-8: Test WithTimeout panics on negative timeout
func TestSecurity_WithTimeout_NegativePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for negative timeout, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "timeout must be positive") {
				t.Errorf("Panic message = %q, want 'timeout must be positive'", msg)
			}
		}
	}()

	_ = New(WithTimeout(-1 * time.Second))
}

// R-8: Test WithMaxConcurrency panics on negative value
func TestSecurity_WithMaxConcurrency_NegativePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for negative max concurrency, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "max concurrency must be non-negative") {
				t.Errorf("Panic message = %q, want 'max concurrency must be non-negative'", msg)
			}
		}
	}()

	_ = New(WithMaxConcurrency(-1))
}

// R-8: Test WithMaxBatchSize panics on negative value
func TestSecurity_WithMaxBatchSize_NegativePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for negative max batch size, got none")
		} else {
			msg := r.(string)
			if !strings.Contains(msg, "max batch size must be non-negative") {
				t.Errorf("Panic message = %q, want 'max batch size must be non-negative'", msg)
			}
		}
	}()

	_ = New(WithMaxBatchSize(-1))
}

// Additional: Test recipe overwrite still works with normal RegisterRecipe
func TestSecurity_RecipeOverwrite(t *testing.T) {
	orch := New()

	handler1 := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "v1", nil
	}

	handler2 := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "v2", nil
	}

	// Register first handler
	orch.RegisterRecipe("test", handler1)

	// Execute and verify v1
	results := orch.ExecuteBatch(context.Background(), []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "test"},
	})

	if results[0].Data != "v1" {
		t.Errorf("First handler returned %v, want 'v1'", results[0].Data)
	}

	// Register second handler (overwrite)
	orch.RegisterRecipe("test", handler2)

	// Execute and verify v2
	results = orch.ExecuteBatch(context.Background(), []SubRequest{
		{ID: "2", TenantID: "tenant-a", Recipe: "test"},
	})

	if results[0].Data != "v2" {
		t.Errorf("Second handler returned %v, want 'v2'", results[0].Data)
	}
}
