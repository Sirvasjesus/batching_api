package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/voseghale/batching"
)

// LoggingHook implements ExecutionHook for structured logging
type LoggingHook struct{}

func (h *LoggingHook) OnStart(ctx context.Context, req relayer.SubRequest) {
	tenantID, _ := relayer.TenantID(ctx)
	log.Printf("[START] tenant=%s recipe=%s id=%s payload=%v",
		tenantID, req.Recipe, req.ID, req.Payload)
}

func (h *LoggingHook) OnComplete(ctx context.Context, req relayer.SubRequest, resp relayer.Response, duration time.Duration) {
	tenantID, _ := relayer.TenantID(ctx)
	status := "SUCCESS"
	if resp.Status >= 400 {
		status = "FAILED"
	}

	log.Printf("[%s] tenant=%s recipe=%s id=%s status=%d duration=%v",
		status, tenantID, req.Recipe, resp.ID, resp.Status, duration)
}

// MetricsHook implements ExecutionHook for collecting metrics
type MetricsHook struct {
	mu              sync.RWMutex
	totalRequests   int
	successRequests int
	failedRequests  int
	totalDuration   time.Duration
	recipeCount     map[string]int
}

func NewMetricsHook() *MetricsHook {
	return &MetricsHook{
		recipeCount: make(map[string]int),
	}
}

func (h *MetricsHook) OnStart(ctx context.Context, req relayer.SubRequest) {
	// Could send to metrics system (Prometheus, StatsD, etc.)
}

func (h *MetricsHook) OnComplete(ctx context.Context, req relayer.SubRequest, resp relayer.Response, duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.totalRequests++
	h.totalDuration += duration

	if resp.Status >= 200 && resp.Status < 300 {
		h.successRequests++
	} else {
		h.failedRequests++
	}

	h.recipeCount[req.Recipe]++
}

func (h *MetricsHook) PrintMetrics() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	fmt.Println("\nMetrics Summary:")
	fmt.Println("================")
	fmt.Printf("Total Requests:    %d\n", h.totalRequests)
	fmt.Printf("Successful:        %d (%.1f%%)\n",
		h.successRequests,
		float64(h.successRequests)/float64(h.totalRequests)*100)
	fmt.Printf("Failed:            %d (%.1f%%)\n",
		h.failedRequests,
		float64(h.failedRequests)/float64(h.totalRequests)*100)

	if h.totalRequests > 0 {
		avgDuration := h.totalDuration / time.Duration(h.totalRequests)
		fmt.Printf("Avg Duration:      %v\n", avgDuration)
	}

	fmt.Println("\nRecipe Usage:")
	for recipe, count := range h.recipeCount {
		fmt.Printf("  %s: %d requests\n", recipe, count)
	}
}

// PanicAlertHook implements PanicHook for alerting on panics
type PanicAlertHook struct{}

func (h *PanicAlertHook) OnPanic(ctx context.Context, req relayer.SubRequest, recovered interface{}) {
	tenantID, _ := relayer.TenantID(ctx)
	log.Printf("⚠️  [PANIC ALERT] tenant=%s recipe=%s id=%s panic=%v",
		tenantID, req.Recipe, req.ID, recovered)

	// In production, send to alerting system (PagerDuty, Slack, etc.)
}

func main() {
	// Create hooks
	loggingHook := &LoggingHook{}
	metricsHook := NewMetricsHook()
	panicHook := &PanicAlertHook{}

	// Create orchestrator with hooks
	orch := relayer.New(
		relayer.WithTimeout(5*time.Second),
		relayer.WithExecutionHook(metricsHook), // Use metrics hook for ExecutionHook
		relayer.WithPanicHook(panicHook),
	)

	// Note: For multiple execution hooks, you could create a CompositeHook
	// For this example, we'll manually call logging in recipes or use metrics only

	// Register recipes
	orch.RegisterRecipe("success", func(ctx context.Context, payload interface{}) (interface{}, error) {
		// Manually log if not using composite hook
		loggingHook.OnStart(ctx, relayer.SubRequest{Recipe: "success"})
		time.Sleep(10 * time.Millisecond) // Simulate work
		return "completed", nil
	})

	orch.RegisterRecipe("slow", func(ctx context.Context, payload interface{}) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "slow result", nil
	}, &relayer.RecipeOption{
		Timeout: 10 * time.Second, // Custom timeout for slow operations
	})

	orch.RegisterRecipe("error", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return nil, fmt.Errorf("intentional error for testing")
	})

	orch.RegisterRecipe("panic", func(ctx context.Context, payload interface{}) (interface{}, error) {
		panic("intentional panic for testing")
	})

	// Create batch with various scenarios
	batch := []relayer.SubRequest{
		{ID: "1", TenantID: "tenant-a", Recipe: "success", Payload: "test1"},
		{ID: "2", TenantID: "tenant-a", Recipe: "success", Payload: "test2"},
		{ID: "3", TenantID: "tenant-b", Recipe: "slow", Payload: "test3"},
		{ID: "4", TenantID: "tenant-b", Recipe: "error", Payload: "test4"},
		{ID: "5", TenantID: "tenant-c", Recipe: "panic", Payload: "test5"},
		{ID: "6", TenantID: "tenant-c", Recipe: "nonexistent", Payload: "test6"},
		{ID: "7", TenantID: "tenant-a", Recipe: "success", Payload: "test7"},
	}

	// Execute batch
	fmt.Println("Executing batch with middleware hooks...")
	fmt.Println("==========================================")
	results := orch.ExecuteBatch(context.Background(), batch)

	// Print results
	fmt.Println("\nResults:")
	fmt.Println("========")
	for _, resp := range results {
		status := "✓"
		if resp.Status >= 400 {
			status = "✗"
		}
		fmt.Printf("%s Request %s: Status=%d Duration=%v\n",
			status, resp.ID, resp.Status, resp.Duration)
	}

	// Print metrics
	metricsHook.PrintMetrics()

	// Demonstrate filtering
	successes := relayer.FilterSuccess(results)
	fmt.Printf("\n\nPartial Success: %d/%d requests succeeded\n",
		len(successes), len(results))

	log.Println("\nMiddleware example completed successfully!")
}
