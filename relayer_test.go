package relayer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	orch := New()

	if orch.timeout != 5*time.Second {
		t.Errorf("Default timeout = %v, want %v", orch.timeout, 5*time.Second)
	}

	if orch.maxConcurrency != 0 {
		t.Errorf("Default maxConcurrency = %d, want 0", orch.maxConcurrency)
	}

	if orch.registry == nil {
		t.Error("Registry not initialized")
	}
}

func TestNew_WithOptions(t *testing.T) {
	hook := &mockExecutionHook{}
	orch := New(
		WithTimeout(10*time.Second),
		WithMaxConcurrency(50),
		WithExecutionHook(hook),
	)

	if orch.timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want %v", orch.timeout, 10*time.Second)
	}

	if orch.maxConcurrency != 50 {
		t.Errorf("MaxConcurrency = %d, want 50", orch.maxConcurrency)
	}

	if orch.executionHook != hook {
		t.Error("ExecutionHook not set correctly")
	}

	if orch.semaphore == nil {
		t.Error("Semaphore not initialized with maxConcurrency > 0")
	}
}

func TestRegisterRecipe_Basic(t *testing.T) {
	orch := New()

	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "result", nil
	}

	orch.RegisterRecipe("test-recipe", handler)

	orch.mu.RLock()
	_, exists := orch.registry["test-recipe"]
	orch.mu.RUnlock()

	if !exists {
		t.Error("Recipe not registered")
	}
}

func TestRegisterRecipe_WithOptions(t *testing.T) {
	orch := New()

	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "result", nil
	}

	orch.RegisterRecipe("test-recipe", handler, &RecipeOption{
		Timeout: 30 * time.Second,
	})

	orch.mu.RLock()
	opt, exists := orch.recipeOptions["test-recipe"]
	orch.mu.RUnlock()

	if !exists {
		t.Error("Recipe options not registered")
	}

	if opt.Timeout != 30*time.Second {
		t.Errorf("Recipe timeout = %v, want %v", opt.Timeout, 30*time.Second)
	}
}

func TestExecuteBatch_BasicSuccess(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("echo", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "echo", Payload: "hello"},
		{ID: "2", TenantID: "tenant-b", Recipe: "echo", Payload: "world"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	if results[0].Status != 200 {
		t.Errorf("Result 0 status = %d, want 200", results[0].Status)
	}

	if results[0].Data != "hello" {
		t.Errorf("Result 0 data = %v, want 'hello'", results[0].Data)
	}

	if results[1].Data != "world" {
		t.Errorf("Result 1 data = %v, want 'world'", results[1].Data)
	}
}

func TestExecuteBatch_MultiTenantIsolation(t *testing.T) {
	orch := New()

	var receivedTenants []string
	var mu sync.Mutex

	orch.RegisterRecipe("capture-tenant", func(ctx context.Context, payload interface{}) (interface{}, error) {
		tenantID, ok := TenantID(ctx)
		if !ok {
			return nil, errors.New("tenant ID not in context")
		}

		mu.Lock()
		receivedTenants = append(receivedTenants, tenantID)
		mu.Unlock()

		return tenantID, nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "capture-tenant"},
		{ID: "2", TenantID: "tenant-b", Recipe: "capture-tenant"},
		{ID: "3", TenantID: "tenant-c", Recipe: "capture-tenant"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	// Verify all tenants were processed
	mu.Lock()
	if len(receivedTenants) != 3 {
		t.Errorf("Received %d tenants, want 3", len(receivedTenants))
	}
	mu.Unlock()

	// Verify tenant isolation in results
	expectedTenants := map[string]bool{
		"tenant-a": true,
		"tenant-b": true,
		"tenant-c": true,
	}

	for _, result := range results {
		if result.Status != 200 {
			t.Errorf("Result status = %d, want 200", result.Status)
		}

		tenantID, ok := result.Data.(string)
		if !ok {
			t.Errorf("Result data is not string: %T", result.Data)
			continue
		}

		if !expectedTenants[tenantID] {
			t.Errorf("Unexpected tenant ID: %s", tenantID)
		}
	}
}

func TestExecuteBatch_RecipeNotFound(t *testing.T) {
	orch := New()

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "nonexistent"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Status != 404 {
		t.Errorf("Status = %d, want 404", results[0].Status)
	}

	if results[0].Error == nil {
		t.Error("Error is nil, want error")
	}

	if results[0].Error.Code != ErrCodeRecipeNotFound {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodeRecipeNotFound)
	}
}

func TestExecuteBatch_RecipeError(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("error-recipe", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, errors.New("something went wrong")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "error-recipe"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 500 {
		t.Errorf("Status = %d, want 500", results[0].Status)
	}

	if results[0].Error == nil {
		t.Error("Error is nil, want error")
	}

	if results[0].Error.Code != ErrCodeRecipeExecution {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodeRecipeExecution)
	}
}

func TestExecuteBatch_Timeout(t *testing.T) {
	orch := New(WithTimeout(50 * time.Millisecond))

	orch.RegisterRecipe("slow", func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "too late", nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "slow"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 504 {
		t.Errorf("Status = %d, want 504 (timeout)", results[0].Status)
	}

	if results[0].Error == nil {
		t.Error("Error is nil, want timeout error")
	}

	if results[0].Error.Code != ErrCodeTimeout {
		t.Errorf("Error code = %s, want %s", results[0].Error.Code, ErrCodeTimeout)
	}
}

func TestExecuteBatch_PerRecipeTimeout(t *testing.T) {
	orch := New(WithTimeout(50 * time.Millisecond))

	orch.RegisterRecipe("slow", func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "completed", nil
	}, &RecipeOption{
		Timeout: 200 * time.Millisecond, // Override with longer timeout
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "slow"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 200 {
		t.Errorf("Status = %d, want 200 (should succeed with longer timeout)", results[0].Status)
	}

	if results[0].Data != "completed" {
		t.Errorf("Data = %v, want 'completed'", results[0].Data)
	}
}

func TestExecuteBatch_PanicRecovery(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("panic-recipe", func(ctx context.Context, payload interface{}) (interface{}, error) {
		panic("intentional panic")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "panic-recipe"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Status != 500 {
		t.Errorf("Status = %d, want 500", results[0].Status)
	}

	if results[0].Error == nil {
		t.Error("Error is nil, want panic error")
	}
}

func TestExecuteBatch_PanicHookCalled(t *testing.T) {
	panicHook := &mockPanicHook{}
	orch := New(WithPanicHook(panicHook))

	orch.RegisterRecipe("panic-recipe", func(ctx context.Context, payload interface{}) (interface{}, error) {
		panic("test panic")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "panic-recipe"},
	}

	orch.ExecuteBatch(context.Background(), batch)

	panicCalls := panicHook.getPanicCalls()
	if len(panicCalls) != 1 {
		t.Errorf("Expected 1 panic hook call, got %d", len(panicCalls))
	}

	if panicCalls[0].recovered != "test panic" {
		t.Errorf("Recovered value = %v, want 'test panic'", panicCalls[0].recovered)
	}
}

func TestExecuteBatch_ExecutionHooksCalled(t *testing.T) {
	hook := &mockExecutionHook{}
	orch := New(WithExecutionHook(hook))

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "result", nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "test"},
		{ID: "2", TenantID: "tenant-b", Recipe: "test"},
	}

	orch.ExecuteBatch(context.Background(), batch)

	startCalls := hook.getStartCalls()
	if len(startCalls) != 2 {
		t.Errorf("Expected 2 OnStart calls, got %d", len(startCalls))
	}

	completeCalls := hook.getCompleteCalls()
	if len(completeCalls) != 2 {
		t.Errorf("Expected 2 OnComplete calls, got %d", len(completeCalls))
	}
}

func TestExecuteBatch_ContextCancellation(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("long-running", func(ctx context.Context, payload interface{}) (interface{}, error) {
		select {
		case <-time.After(5 * time.Second):
			return "completed", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "long-running"},
	}

	results := orch.ExecuteBatch(ctx, batch)

	// Should get timeout error
	if results[0].Status == 200 {
		t.Error("Expected non-200 status due to context cancellation")
	}
}

func TestExecuteBatch_Concurrency(t *testing.T) {
	orch := New()

	var counter int32

	orch.RegisterRecipe("increment", func(ctx context.Context, payload interface{}) (interface{}, error) {
		newValue := atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond)
		return newValue, nil
	})

	// Create batch of 10 requests
	batch := make([]SubRequest, 10)
	for i := 0; i < 10; i++ {
		batch[i] = SubRequest{
			ID:       fmt.Sprintf("%d", i),
			TenantID: "tenant-a",
			Recipe:   "increment",
		}
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}

	// All should succeed
	for _, result := range results {
		if result.Status != 200 {
			t.Errorf("Result status = %d, want 200", result.Status)
		}
	}

	// Counter should be 10
	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("Counter = %d, want 10", counter)
	}
}

func TestExecuteBatch_MaxConcurrency(t *testing.T) {
	orch := New(WithMaxConcurrency(2))

	var concurrentCalls int32
	var maxConcurrent int32

	orch.RegisterRecipe("monitor-concurrency", func(ctx context.Context, payload interface{}) (interface{}, error) {
		current := atomic.AddInt32(&concurrentCalls, 1)
		defer atomic.AddInt32(&concurrentCalls, -1)

		// Track max concurrency
		for {
			max := atomic.LoadInt32(&maxConcurrent)
			if current <= max {
				break
			}
			if atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		return "done", nil
	})

	// Create batch of 10 requests
	batch := make([]SubRequest, 10)
	for i := 0; i < 10; i++ {
		batch[i] = SubRequest{
			ID:       fmt.Sprintf("%d", i),
			TenantID: "tenant-a",
			Recipe:   "monitor-concurrency",
		}
	}

	orch.ExecuteBatch(context.Background(), batch)

	max := atomic.LoadInt32(&maxConcurrent)
	if max > 2 {
		t.Errorf("Max concurrent calls = %d, want <= 2", max)
	}
}

func TestExecuteBatch_MixedResults(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("success", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return "ok", nil
	})

	orch.RegisterRecipe("error", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, errors.New("failed")
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "success"},
		{ID: "2", TenantID: "tenant-a", Recipe: "error"},
		{ID: "3", TenantID: "tenant-b", Recipe: "success"},
		{ID: "4", TenantID: "tenant-b", Recipe: "nonexistent"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

	// Verify mixed results
	if results[0].Status != 200 {
		t.Errorf("Result 0 status = %d, want 200", results[0].Status)
	}

	if results[1].Status != 500 {
		t.Errorf("Result 1 status = %d, want 500", results[1].Status)
	}

	if results[2].Status != 200 {
		t.Errorf("Result 2 status = %d, want 200", results[2].Status)
	}

	if results[3].Status != 404 {
		t.Errorf("Result 3 status = %d, want 404", results[3].Status)
	}

	// Test partial success filter
	successes := FilterSuccess(results)
	if len(successes) != 2 {
		t.Errorf("FilterSuccess returned %d results, want 2", len(successes))
	}
}

func TestExecuteBatch_EmptyBatch(t *testing.T) {
	orch := New()

	results := orch.ExecuteBatch(context.Background(), []SubRequest{})

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty batch, got %d", len(results))
	}
}

func TestExecuteBatch_ResponseDuration(t *testing.T) {
	orch := New()

	orch.RegisterRecipe("timed", func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return "done", nil
	})

	batch := []SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "timed"},
	}

	results := orch.ExecuteBatch(context.Background(), batch)

	if results[0].Duration < 50*time.Millisecond {
		t.Errorf("Duration = %v, want >= 50ms", results[0].Duration)
	}
}
