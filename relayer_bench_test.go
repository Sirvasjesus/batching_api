package relayer

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkExecuteBatch_100(b *testing.B) {
	benchmarkExecuteBatch(b, 100)
}

func BenchmarkExecuteBatch_1000(b *testing.B) {
	benchmarkExecuteBatch(b, 1000)
}

func BenchmarkExecuteBatch_10000(b *testing.B) {
	benchmarkExecuteBatch(b, 10000)
}

func benchmarkExecuteBatch(b *testing.B, batchSize int) {
	orch := New(WithTimeout(30 * time.Second))

	// Register a simple echo recipe
	orch.RegisterRecipe("echo", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	// Prepare batch
	batch := make([]SubRequest, batchSize)
	for i := 0; i < batchSize; i++ {
		batch[i] = SubRequest{
			ID:       fmt.Sprintf("req-%d", i),
			TenantID: fmt.Sprintf("tenant-%d", i%10), // 10 tenants
			Recipe:   "echo",
			Payload:  fmt.Sprintf("data-%d", i),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results := orch.ExecuteBatch(context.Background(), batch)
		if len(results) != batchSize {
			b.Fatalf("Expected %d results, got %d", batchSize, len(results))
		}
	}
}

func BenchmarkFilterSuccess(b *testing.B) {
	// Prepare responses
	responses := make([]Response, 1000)
	for i := 0; i < 1000; i++ {
		status := 200
		if i%5 == 0 {
			status = 404
		}
		if i%7 == 0 {
			status = 500
		}
		responses[i] = Response{
			ID:     fmt.Sprintf("req-%d", i),
			Status: status,
			Data:   "data",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		successes := FilterSuccess(responses)
		_ = successes
	}
}

func BenchmarkContextOperations(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx = WithTenantID(ctx, "tenant-123")
		ctx = WithRequestID(ctx, "req-456")
		ctx = WithRecipeName(ctx, "recipe-789")

		_, _ = TenantID(ctx)
		_, _ = RequestID(ctx)
		_, _ = RecipeName(ctx)
	}
}

func BenchmarkRegisterRecipe(b *testing.B) {
	handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		orch := New()
		orch.RegisterRecipe("test-recipe", handler)
	}
}

func BenchmarkConcurrentExecution_NoLimit(b *testing.B) {
	orch := New()

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	batch := make([]SubRequest, 100)
	for i := 0; i < 100; i++ {
		batch[i] = SubRequest{
			ID:       fmt.Sprintf("req-%d", i),
			TenantID: "tenant-a",
			Recipe:   "test",
			Payload:  i,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		orch.ExecuteBatch(context.Background(), batch)
	}
}

func BenchmarkConcurrentExecution_WithLimit(b *testing.B) {
	orch := New(WithMaxConcurrency(10))

	orch.RegisterRecipe("test", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	batch := make([]SubRequest, 100)
	for i := 0; i < 100; i++ {
		batch[i] = SubRequest{
			ID:       fmt.Sprintf("req-%d", i),
			TenantID: "tenant-a",
			Recipe:   "test",
			Payload:  i,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		orch.ExecuteBatch(context.Background(), batch)
	}
}
