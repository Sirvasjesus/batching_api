package relayer

import (
	"context"
	"sync"
	"testing"
	"time"
)

// Mock hook for testing
type mockExecutionHook struct {
	mu          sync.Mutex
	startCalls  []SubRequest
	completeCalls []completeCall
}

type completeCall struct {
	req      SubRequest
	resp     Response
	duration time.Duration
}

func (h *mockExecutionHook) OnStart(ctx context.Context, req SubRequest) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.startCalls = append(h.startCalls, req)
}

func (h *mockExecutionHook) OnComplete(ctx context.Context, req SubRequest, resp Response, duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.completeCalls = append(h.completeCalls, completeCall{
		req:      req,
		resp:     resp,
		duration: duration,
	})
}

func (h *mockExecutionHook) getStartCalls() []SubRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]SubRequest{}, h.startCalls...)
}

func (h *mockExecutionHook) getCompleteCalls() []completeCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]completeCall{}, h.completeCalls...)
}

// Mock panic hook for testing
type mockPanicHook struct {
	mu         sync.Mutex
	panicCalls []panicCall
}

type panicCall struct {
	req       SubRequest
	recovered interface{}
}

func (h *mockPanicHook) OnPanic(ctx context.Context, req SubRequest, recovered interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.panicCalls = append(h.panicCalls, panicCall{
		req:       req,
		recovered: recovered,
	})
}

func (h *mockPanicHook) getPanicCalls() []panicCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]panicCall{}, h.panicCalls...)
}

func TestExecutionHook_Interface(t *testing.T) {
	var _ ExecutionHook = (*mockExecutionHook)(nil)
	var _ ExecutionHook = (*NoOpHook)(nil)
}

func TestPanicHook_Interface(t *testing.T) {
	var _ PanicHook = (*mockPanicHook)(nil)
	var _ PanicHook = (*NoOpHook)(nil)
}

func TestNoOpHook_DoesNotPanic(t *testing.T) {
	hook := &NoOpHook{}
	ctx := context.Background()
	req := SubRequest{ID: "test", TenantID: "tenant-1", Recipe: "test-recipe"}
	resp := Response{ID: "test", Status: 200}

	// Should not panic
	hook.OnStart(ctx, req)
	hook.OnComplete(ctx, req, resp, time.Millisecond)
	hook.OnPanic(ctx, req, "panic message")
}

func TestMockExecutionHook_RecordsCallbacks(t *testing.T) {
	hook := &mockExecutionHook{}
	ctx := context.Background()

	req1 := SubRequest{ID: "1", TenantID: "tenant-a", Recipe: "recipe-1"}
	req2 := SubRequest{ID: "2", TenantID: "tenant-b", Recipe: "recipe-2"}
	resp1 := Response{ID: "1", Status: 200}
	resp2 := Response{ID: "2", Status: 404}

	hook.OnStart(ctx, req1)
	hook.OnStart(ctx, req2)
	hook.OnComplete(ctx, req1, resp1, 10*time.Millisecond)
	hook.OnComplete(ctx, req2, resp2, 20*time.Millisecond)

	startCalls := hook.getStartCalls()
	if len(startCalls) != 2 {
		t.Errorf("Expected 2 OnStart calls, got %d", len(startCalls))
	}

	completeCalls := hook.getCompleteCalls()
	if len(completeCalls) != 2 {
		t.Errorf("Expected 2 OnComplete calls, got %d", len(completeCalls))
	}

	if completeCalls[0].resp.Status != 200 {
		t.Errorf("First complete call status = %d, want 200", completeCalls[0].resp.Status)
	}

	if completeCalls[1].resp.Status != 404 {
		t.Errorf("Second complete call status = %d, want 404", completeCalls[1].resp.Status)
	}
}

func TestMockPanicHook_RecordsCallbacks(t *testing.T) {
	hook := &mockPanicHook{}
	ctx := context.Background()

	req1 := SubRequest{ID: "1", TenantID: "tenant-a", Recipe: "recipe-1"}
	req2 := SubRequest{ID: "2", TenantID: "tenant-b", Recipe: "recipe-2"}

	hook.OnPanic(ctx, req1, "panic message 1")
	hook.OnPanic(ctx, req2, "panic message 2")

	panicCalls := hook.getPanicCalls()
	if len(panicCalls) != 2 {
		t.Errorf("Expected 2 OnPanic calls, got %d", len(panicCalls))
	}

	if panicCalls[0].recovered != "panic message 1" {
		t.Errorf("First panic recovered = %v, want 'panic message 1'", panicCalls[0].recovered)
	}

	if panicCalls[1].req.ID != "2" {
		t.Errorf("Second panic req.ID = %s, want '2'", panicCalls[1].req.ID)
	}
}

func TestHooks_ThreadSafety(t *testing.T) {
	hook := &mockExecutionHook{}
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := SubRequest{ID: string(rune(id)), TenantID: "tenant", Recipe: "recipe"}
			resp := Response{ID: string(rune(id)), Status: 200}
			hook.OnStart(ctx, req)
			hook.OnComplete(ctx, req, resp, time.Millisecond)
		}(i)
	}
	wg.Wait()

	startCalls := hook.getStartCalls()
	if len(startCalls) != 100 {
		t.Errorf("Expected 100 OnStart calls, got %d", len(startCalls))
	}

	completeCalls := hook.getCompleteCalls()
	if len(completeCalls) != 100 {
		t.Errorf("Expected 100 OnComplete calls, got %d", len(completeCalls))
	}
}
