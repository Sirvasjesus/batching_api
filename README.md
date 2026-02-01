# Relayer: Multi-Tenant Northbound Batcher

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen.svg)](.)

**Relayer** is an ultrathin, production-ready Go library for high-efficiency batch processing with multi-tenant isolation. It provides a simple yet powerful orchestration mechanism for executing parallel requests while maintaining strict tenant boundaries.

## Features

✨ **Ultrathin Design** - Zero external dependencies, stdlib only, <2000 LOC
🔒 **Multi-Tenant Isolation** - Type-safe context-based tenant separation
⚡ **High Performance** - Goroutine-based parallel execution
🎯 **Partial Success** - Returns all successful results, isolates failures
🛡️ **Production Ready** - Panic recovery, timeout handling, 100% test coverage
🔌 **Transport Agnostic** - Works with HTTP, gRPC, QUIC, or any protocol
📊 **Observable** - Built-in hooks for logging, metrics, and tracing
⚙️ **Configurable** - Options pattern for flexible configuration

## Quick Start

### Installation

```bash
go get github.com/voseghale/batching
```

### 5-Minute Example

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/voseghale/batching"
)

func main() {
    // Create orchestrator
    orch := relayer.New(
        relayer.WithTimeout(5 * time.Second),
    )

    // Register a recipe (business logic)
    orch.RegisterRecipe("greet", func(ctx context.Context, payload interface{}) (interface{}, error) {
        name := payload.(string)
        tenantID, _ := relayer.TenantID(ctx)
        return fmt.Sprintf("Hello %s from %s!", name, tenantID), nil
    })

    // Create batch of requests
    batch := []relayer.SubRequest{
        {ID: "1", TenantID: "company-a", Recipe: "greet", Payload: "Alice"},
        {ID: "2", TenantID: "company-b", Recipe: "greet", Payload: "Bob"},
    }

    // Execute batch
    results := orch.ExecuteBatch(context.Background(), batch)

    // Filter successes (partial success pattern)
    successes := relayer.FilterSuccess(results)
    for _, resp := range successes {
        fmt.Printf("%s: %v\n", resp.ID, resp.Data)
    }
}
```

## Core Concepts

### Multi-Tenancy

Every request is associated with a tenant ID, injected into the context for secure isolation:

```go
orch.RegisterRecipe("get-user", func(ctx context.Context, payload interface{}) (interface{}, error) {
    // CRITICAL: Always extract tenant ID for scoped queries
    tenantID, _ := relayer.TenantID(ctx)

    // Perform tenant-scoped database query
    user := db.Where("tenant_id = ?", tenantID).First(...)
    return user, nil
})
```

### Recipes

A **recipe** is simply a function that processes a payload and returns data or an error:

```go
type Handler func(ctx context.Context, payload interface{}) (interface{}, error)
```

Recipes are:
- **Tenant-aware** via context
- **Registered** by name with the orchestrator
- **Executed** in parallel goroutines
- **Isolated** - one recipe's panic doesn't affect others

### Partial Success

The orchestrator returns results for ALL requests, allowing you to filter:

```go
results := orch.ExecuteBatch(ctx, batch)

// Get only successes (2xx status codes)
successes := relayer.FilterSuccess(results)

// Get only errors
errors := relayer.FilterByStatus(results, 500)

// Get results for specific tenant
tenantResults := relayer.FilterByTenant(results, "company-a")
```

### Request/Response Model

```go
// Input
type SubRequest struct {
    ID       string      // Unique request identifier
    TenantID string      // Tenant for isolation
    Recipe   string      // Recipe name to execute
    Payload  interface{} // Any JSON-serializable data
}

// Output
type Response struct {
    ID       string        // Matches SubRequest.ID
    Status   int           // HTTP-style status code
    Data     interface{}   // Result data (if successful)
    Error    *Error        // Error details (if failed)
    Duration time.Duration // Execution time
    TenantID string        // Tenant ID for filtering
}
```

## API Reference

### Orchestrator Lifecycle

```go
// Create new orchestrator
orch := relayer.New(opts ...Option)

// Register a recipe
orch.RegisterRecipe(name string, handler Handler, opts ...*RecipeOption)

// Execute batch
results := orch.ExecuteBatch(ctx context.Context, batch []SubRequest) []Response
```

### Configuration Options

```go
// Set default timeout
relayer.WithTimeout(duration time.Duration)

// Limit concurrent executions
relayer.WithMaxConcurrency(max int)

// Add execution hook for logging/metrics
relayer.WithExecutionHook(hook ExecutionHook)

// Add panic recovery hook
relayer.WithPanicHook(hook PanicHook)
```

### Per-Recipe Configuration

```go
orch.RegisterRecipe("slow-operation", handler, &relayer.RecipeOption{
    Timeout: 30 * time.Second, // Override default timeout
})
```

### Response Filtering

```go
// Get successful responses (2xx status codes)
relayer.FilterSuccess(responses []Response) []Response

// Get responses with specific status
relayer.FilterByStatus(responses []Response, status int) []Response

// Get responses for specific tenant
relayer.FilterByTenant(responses []Response, tenantID string) []Response
```

### Context Helpers

```go
// Extract tenant ID (type-safe)
tenantID, ok := relayer.TenantID(ctx)

// Extract request ID
requestID, ok := relayer.RequestID(ctx)

// Extract recipe name
recipeName, ok := relayer.RecipeName(ctx)
```

## Advanced Usage

### Observability Hooks

```go
type LoggingHook struct{}

func (h *LoggingHook) OnStart(ctx context.Context, req relayer.SubRequest) {
    log.Printf("Starting: tenant=%s recipe=%s", req.TenantID, req.Recipe)
}

func (h *LoggingHook) OnComplete(ctx context.Context, req relayer.SubRequest,
    resp relayer.Response, duration time.Duration) {
    log.Printf("Completed: id=%s status=%d duration=%v",
        resp.ID, resp.Status, duration)
}

// Use the hook
orch := relayer.New(
    relayer.WithExecutionHook(&LoggingHook{}),
)
```

### Panic Recovery

```go
type AlertingHook struct{}

func (h *AlertingHook) OnPanic(ctx context.Context, req relayer.SubRequest, recovered interface{}) {
    tenantID, _ := relayer.TenantID(ctx)
    alert.Send("Recipe panic: tenant=%s recipe=%s error=%v",
        tenantID, req.Recipe, recovered)
}

orch := relayer.New(
    relayer.WithPanicHook(&AlertingHook{}),
)
```

### Concurrency Control

```go
// Limit to 50 concurrent recipe executions
orch := relayer.New(
    relayer.WithMaxConcurrency(50),
)
```

## Transport Integration

### HTTP Server

```go
func handleBatch(w http.ResponseWriter, r *http.Request, orch *relayer.Orchestrator) {
    var batch []relayer.SubRequest
    json.NewDecoder(r.Body).Decode(&batch)

    results := orch.ExecuteBatch(r.Context(), batch)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "results": results,
        "summary": map[string]int{
            "total":     len(results),
            "successes": len(relayer.FilterSuccess(results)),
        },
    })
}
```

See [examples/http-server](examples/http-server) for complete implementation.

### gRPC Server

```go
func (s *Server) ExecuteBatch(ctx context.Context, req *pb.BatchRequest) (*pb.BatchResponse, error) {
    // Convert protobuf to relayer types
    batch := convertProtoToBatch(req)

    // Execute
    results := s.orch.ExecuteBatch(ctx, batch)

    // Convert back to protobuf
    return convertToProtoResponse(results), nil
}
```

See [examples/grpc-server](examples/grpc-server) for complete implementation with `.proto` file.

### QUIC Server

QUIC provides low-latency, multiplexed connections perfect for batch processing:

```go
func handleStream(stream quic.Stream, orch *relayer.Orchestrator) {
    var batch []relayer.SubRequest
    json.NewDecoder(stream).Decode(&batch)

    results := orch.ExecuteBatch(context.Background(), batch)

    json.NewEncoder(stream).Encode(results)
}
```

See [examples/quic-server](examples/quic-server) for complete implementation.

## Best Practices

### Security: Tenant Isolation

**✅ DO**: Always extract tenant ID from context

```go
func MyRecipe(ctx context.Context, payload interface{}) (interface{}, error) {
    tenantID, ok := relayer.TenantID(ctx)
    if !ok {
        return nil, errors.New("tenant ID required")
    }

    // Use tenantID in scoped queries
    data := db.Where("tenant_id = ?", tenantID).Find(...)
    return data, nil
}
```

**❌ DON'T**: Trust tenant ID from payload

```go
// INSECURE! User could spoof tenant ID in payload
func BadRecipe(ctx context.Context, payload interface{}) (interface{}, error) {
    tenantID := payload.(map[string]interface{})["tenant_id"]
    // ... dangerous!
}
```

### Error Handling

Return descriptive errors:

```go
func MyRecipe(ctx context.Context, payload interface{}) (interface{}, error) {
    data, ok := payload.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("payload must be object, got %T", payload)
    }

    // ... process

    if err != nil {
        return nil, fmt.Errorf("failed to process: %w", err)
    }

    return result, nil
}
```

### Performance Tuning

**Timeouts**: Set appropriate timeouts

```go
orch := relayer.New(
    relayer.WithTimeout(5 * time.Second), // Default for fast operations
)

orch.RegisterRecipe("slow-job", handler, &relayer.RecipeOption{
    Timeout: 30 * time.Second, // Override for slow operations
})
```

**Concurrency Limiting**: Prevent resource exhaustion

```go
orch := relayer.New(
    relayer.WithMaxConcurrency(100), // Limit based on system resources
)
```

**Batch Sizing**: Limit batch sizes at transport layer

```go
if len(batch) > 1000 {
    return http.Error(w, "Batch too large", http.StatusBadRequest)
}
```

### Testing Recipes

```go
func TestMyRecipe(t *testing.T) {
    // Create context with tenant ID
    ctx := relayer.WithTenantID(context.Background(), "test-tenant")

    // Execute recipe
    result, err := MyRecipe(ctx, testPayload)

    // Verify
    if err != nil {
        t.Fatalf("Recipe failed: %v", err)
    }

    // Check result
    // ...
}
```

## Error Codes

The library uses HTTP-style status codes:

| Status | Code | Meaning |
|--------|------|---------|
| 200 | - | Success |
| 404 | `RECIPE_NOT_FOUND` | Recipe name not registered |
| 500 | `RECIPE_EXECUTION` | Recipe returned error |
| 500 | `PANIC` | Recipe panicked |
| 504 | `TIMEOUT` | Recipe execution timed out |

## Architecture

See [../documentation/architecture.md](../documentation/architecture.md) for detailed architecture documentation.

Key design principles:
- **Ultrathin**: Stdlib only, minimal API surface
- **Transport Agnostic**: Core library knows nothing about HTTP/gRPC/QUIC
- **Hook-Based Observability**: Provide interfaces, not implementations
- **Type-Safe Contexts**: Prevent key collisions with typed context keys
- **Panic Safety**: All goroutines have panic recovery
- **Concurrent Execution**: WaitGroup ensures batch completes before returning

## Examples

Complete examples available in [examples/](examples/):

- **[basic](examples/basic)** - Simple usage with multiple recipes
- **[multitenant](examples/multitenant)** - Multi-tenant database access pattern
- **[middleware](examples/middleware)** - Logging, metrics, and panic hooks
- **[http-server](examples/http-server)** - HTTP REST API integration
- **[grpc-server](examples/grpc-server)** - gRPC service integration
- **[quic-server](examples/quic-server)** - QUIC/HTTP3 integration

## Performance

Benchmarks on AMD RYZEN AI MAX PRO 390:

```
BenchmarkExecuteBatch_100      8415 ns/op    65845 B/op    1102 allocs/op
BenchmarkExecuteBatch_1000   1281788 ns/op   650084 B/op   11005 allocs/op
BenchmarkExecuteBatch_10000 11072082 ns/op  6482110 B/op  110007 allocs/op
```

- Handles 10,000 requests in ~11ms (~900,000 req/sec)
- Linear scaling with batch size
- Low memory footprint (~650 bytes per request)

## Contributing

Contributions are welcome! Please:

1. Run tests: `go test -v -race ./...`
2. Check coverage: `go test -coverprofile=coverage.out ./...`
3. Run benchmarks: `go test -bench=. ./...`
4. Follow Go best practices and maintain 100% coverage

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by the need for efficient multi-tenant batch processing in microservices architectures.

Built with ❤️ using only Go's standard library.
