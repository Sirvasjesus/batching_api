package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/voseghale/batching"
)

// Simulated tenant-scoped database
type TenantDatabase struct {
	mu   sync.RWMutex
	data map[string]map[string]interface{} // tenant_id -> key -> value
}

func NewTenantDatabase() *TenantDatabase {
	return &TenantDatabase{
		data: make(map[string]map[string]interface{}),
	}
}

func (db *TenantDatabase) Set(tenantID, key string, value interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.data[tenantID] == nil {
		db.data[tenantID] = make(map[string]interface{})
	}
	db.data[tenantID][key] = value
}

func (db *TenantDatabase) Get(tenantID, key string) (interface{}, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if tenantData, exists := db.data[tenantID]; exists {
		value, ok := tenantData[key]
		return value, ok
	}
	return nil, false
}

func main() {
	// Create tenant-scoped database
	db := NewTenantDatabase()

	// Seed some data
	db.Set("company-a", "user-1", map[string]string{"name": "Alice", "email": "alice@company-a.com"})
	db.Set("company-a", "user-2", map[string]string{"name": "Bob", "email": "bob@company-a.com"})
	db.Set("company-b", "user-1", map[string]string{"name": "Charlie", "email": "charlie@company-b.com"})
	db.Set("company-b", "user-2", map[string]string{"name": "Diana", "email": "diana@company-b.com"})

	// Create orchestrator
	orch := relayer.New(
		relayer.WithTimeout(5 * time.Second),
	)

	// Register tenant-aware recipe for fetching users
	// IMPORTANT: Always extract tenant ID from context to ensure isolation
	orch.RegisterRecipe("get-user", func(ctx context.Context, payload interface{}) (interface{}, error) {
		// Step 1: Extract tenant ID from context (CRITICAL for security!)
		tenantID, ok := relayer.TenantID(ctx)
		if !ok {
			return nil, fmt.Errorf("tenant ID not found in context")
		}

		userID, ok := payload.(string)
		if !ok {
			return nil, fmt.Errorf("payload must be string user ID")
		}

		// Step 2: Perform tenant-scoped query
		userData, found := db.Get(tenantID, userID)
		if !found {
			return nil, fmt.Errorf("user %s not found for tenant %s", userID, tenantID)
		}

		return userData, nil
	})

	// Create batch with requests from multiple tenants
	batch := []relayer.SubRequest{
		{ID: "req-1", TenantID: "company-a", Recipe: "get-user", Payload: "user-1"},
		{ID: "req-2", TenantID: "company-a", Recipe: "get-user", Payload: "user-2"},
		{ID: "req-3", TenantID: "company-b", Recipe: "get-user", Payload: "user-1"},
		{ID: "req-4", TenantID: "company-b", Recipe: "get-user", Payload: "user-2"},
		{ID: "req-5", TenantID: "company-a", Recipe: "get-user", Payload: "user-999"}, // Not found
		{ID: "req-6", TenantID: "company-c", Recipe: "get-user", Payload: "user-1"},   // Wrong tenant
	}

	// Execute batch
	fmt.Println("Executing multi-tenant batch...")
	results := orch.ExecuteBatch(context.Background(), batch)

	// Print results grouped by tenant
	fmt.Println("\nResults by Tenant:")
	fmt.Println("==================")

	for _, tenantID := range []string{"company-a", "company-b", "company-c"} {
		fmt.Printf("\n%s:\n", tenantID)
		tenantResults := relayer.FilterByTenant(results, tenantID)

		for _, resp := range tenantResults {
			fmt.Printf("  Request %s: ", resp.ID)
			if resp.Error != nil {
				fmt.Printf("ERROR - %s\n", resp.Error.Message)
			} else {
				fmt.Printf("SUCCESS - %v\n", resp.Data)
			}
		}
	}

	// Demonstrate tenant isolation
	fmt.Println("\n\nTenant Isolation Demonstration:")
	fmt.Println("================================")
	fmt.Println("Notice that:")
	fmt.Println("1. company-a can only access company-a's users")
	fmt.Println("2. company-b can only access company-b's users")
	fmt.Println("3. company-c gets 'not found' errors (no data in system)")
	fmt.Println("4. One tenant's failure doesn't affect another tenant's requests")

	// Show partial success pattern
	successes := relayer.FilterSuccess(results)
	fmt.Printf("\nPartial Success: %d/%d requests succeeded\n", len(successes), len(results))

	log.Println("\nMulti-tenant example completed successfully!")
}
