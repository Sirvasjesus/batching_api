package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/voseghale/batching"
)

func main() {
	// Create a new orchestrator with 5-second timeout
	orch := relayer.New(
		relayer.WithTimeout(5 * time.Second),
	)

	// Register a simple greeting recipe
	orch.RegisterRecipe("greet", func(ctx context.Context, payload interface{}) (interface{}, error) {
		name, ok := payload.(string)
		if !ok {
			return nil, fmt.Errorf("payload must be string, got %T", payload)
		}

		tenantID, _ := relayer.TenantID(ctx)
		return fmt.Sprintf("Hello %s from tenant %s!", name, tenantID), nil
	})

	// Register a math recipe
	orch.RegisterRecipe("add", func(ctx context.Context, payload interface{}) (interface{}, error) {
		numbers, ok := payload.([]interface{})
		if !ok || len(numbers) != 2 {
			return nil, fmt.Errorf("payload must be array of 2 numbers")
		}

		a, ok1 := numbers[0].(float64)
		b, ok2 := numbers[1].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("both values must be numbers")
		}

		return a + b, nil
	})

	// Create a batch of requests
	batch := []relayer.SubRequest{
		{
			ID:       "req-1",
			TenantID: "tenant-alice",
			Recipe:   "greet",
			Payload:  "Alice",
		},
		{
			ID:       "req-2",
			TenantID: "tenant-bob",
			Recipe:   "greet",
			Payload:  "Bob",
		},
		{
			ID:       "req-3",
			TenantID: "tenant-math",
			Recipe:   "add",
			Payload:  []interface{}{10.0, 32.0},
		},
		{
			ID:       "req-4",
			TenantID: "tenant-test",
			Recipe:   "nonexistent",
			Payload:  "test",
		},
	}

	// Execute the batch
	fmt.Println("Executing batch...")
	results := orch.ExecuteBatch(context.Background(), batch)

	// Print all results
	fmt.Println("\nAll Results:")
	fmt.Println("============")
	for _, resp := range results {
		fmt.Printf("ID: %s, Status: %d, ", resp.ID, resp.Status)
		if resp.Error != nil {
			fmt.Printf("Error: %s\n", resp.Error.Message)
		} else {
			fmt.Printf("Data: %v, Duration: %v\n", resp.Data, resp.Duration)
		}
	}

	// Filter and print only successful results
	successes := relayer.FilterSuccess(results)
	fmt.Println("\nSuccessful Results Only:")
	fmt.Println("========================")
	for _, resp := range successes {
		fmt.Printf("ID: %s, Data: %v\n", resp.ID, resp.Data)
	}

	// Filter by tenant
	tenantResults := relayer.FilterByTenant(results, "tenant-alice")
	fmt.Println("\nResults for tenant-alice:")
	fmt.Println("=========================")
	for _, resp := range tenantResults {
		fmt.Printf("ID: %s, Status: %d, Data: %v\n", resp.ID, resp.Status, resp.Data)
	}

	log.Println("\nBasic example completed successfully!")
}
