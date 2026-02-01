package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/voseghale/batching"
)

/*
 * QUIC Server Example for Relayer Batch Library
 *
 * This example demonstrates how to integrate the relayer library with QUIC protocol.
 * QUIC (Quick UDP Internet Connections) is a modern transport protocol that provides
 * low-latency, multiplexed connections over UDP.
 *
 * To use this example:
 * 1. Install quic-go library:
 *    go get github.com/quic-go/quic-go
 * 2. Generate TLS certificates (QUIC requires TLS):
 *    openssl req -x509 -newkey rsa:2048 -nodes -keyout server.key -out server.crt -days 365
 * 3. Run:
 *    go run main.go
 * 4. Test with QUIC client:
 *    See client example or use curl with HTTP/3:
 *    curl --http3 -k https://localhost:4433/batch -d @batch.json
 *
 * Note: This file provides implementation structure. You'll need to install
 * quic-go before this can be compiled and run.
 */

type QUICBatchServer struct {
	orchestrator *relayer.Orchestrator
}

func NewQUICBatchServer() *QUICBatchServer {
	orch := relayer.New(
		relayer.WithTimeout(5 * time.Second),
		relayer.WithMaxConcurrency(200), // QUIC handles many concurrent streams well
	)

	setupRecipes(orch)

	return &QUICBatchServer{
		orchestrator: orch,
	}
}

func setupRecipes(orch *relayer.Orchestrator) {
	// Low-latency echo - perfect for QUIC
	orch.RegisterRecipe("echo", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	// Fast data processing
	orch.RegisterRecipe("process", func(ctx context.Context, payload interface{}) (interface{}, error) {
		tenantID, _ := relayer.TenantID(ctx)

		return map[string]interface{}{
			"tenant_id":  tenantID,
			"processed":  true,
			"data":       payload,
			"timestamp":  time.Now().UnixMilli(),
			"protocol":   "QUIC",
		}, nil
	})

	// Streaming data aggregation
	orch.RegisterRecipe("aggregate", func(ctx context.Context, payload interface{}) (interface{}, error) {
		data, ok := payload.([]interface{})
		if !ok {
			return nil, fmt.Errorf("payload must be array")
		}

		sum := 0.0
		count := len(data)

		for _, item := range data {
			if num, ok := item.(float64); ok {
				sum += num
			}
		}

		return map[string]interface{}{
			"count":   count,
			"sum":     sum,
			"average": sum / float64(count),
		}, nil
	})

	log.Println("QUIC server recipes registered")
}

// HandleStream processes a QUIC stream
// func (s *QUICBatchServer) HandleStream(stream quic.Stream) {
func HandleStreamExample(orch *relayer.Orchestrator) {
	/*
	// Example implementation after quic-go is installed:

	defer stream.Close()

	// Read request
	var batch []relayer.SubRequest
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&batch); err != nil {
		log.Printf("Failed to decode batch: %v", err)
		return
	}

	// Execute batch
	results := s.orchestrator.ExecuteBatch(context.Background(), batch)

	// Write response
	encoder := json.NewEncoder(stream)
	response := map[string]interface{}{
		"results": results,
		"summary": map[string]interface{}{
			"total":     len(results),
			"successes": len(relayer.FilterSuccess(results)),
			"protocol":  "QUIC",
		},
	}

	if err := encoder.Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
	*/
}

func main() {
	log.Println("QUIC Server Example")
	log.Println("===================")
	log.Println()
	log.Println("This example demonstrates QUIC integration with the batching library.")
	log.Println("QUIC provides:")
	log.Println("  - Low latency (0-RTT connection establishment)")
	log.Println("  - Multiplexing without head-of-line blocking")
	log.Println("  - Built-in encryption (TLS 1.3)")
	log.Println("  - Connection migration (switch networks seamlessly)")
	log.Println()
	log.Println("To use this example:")
	log.Println("1. Install quic-go:")
	log.Println("   go get github.com/quic-go/quic-go")
	log.Println()
	log.Println("2. Generate TLS certificates:")
	log.Println("   openssl req -x509 -newkey rsa:2048 -nodes \\")
	log.Println("     -keyout server.key -out server.crt -days 365 \\")
	log.Println("     -subj '/CN=localhost'")
	log.Println()
	log.Println("3. Uncomment the server code and run:")
	log.Println("   go run main.go")
	log.Println()
	log.Println("Example usage with HTTP/3 (built on QUIC):")
	log.Println("   curl --http3 -k https://localhost:4433 -d @batch.json")
	log.Println()
	log.Println("Sample batch.json:")
	printSampleBatch()
	log.Println()
	log.Println("Why QUIC is great for batching:")
	log.Println("  - Multiple batch requests can be sent concurrently")
	log.Println("  - No head-of-line blocking between different batches")
	log.Println("  - Faster reconnection if network changes")
	log.Println("  - Better performance on lossy networks")

	// Uncomment after installing quic-go:
	/*
	server := NewQUICBatchServer()

	// Load TLS certificates
	tlsCert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Fatalf("Failed to load TLS certificates: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"batch-quic"},
	}

	quicConfig := &quic.Config{
		MaxIdleTimeout:  time.Minute,
		KeepAlivePeriod: 30 * time.Second,
	}

	listener, err := quic.ListenAddr("localhost:4433", tlsConfig, quicConfig)
	if err != nil {
		log.Fatalf("Failed to start QUIC listener: %v", err)
	}

	log.Println("QUIC server listening on localhost:4433")

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go func() {
			for {
				stream, err := conn.AcceptStream(context.Background())
				if err != nil {
					return
				}
				go server.HandleStream(stream)
			}
		}()
	}
	*/
}

func printSampleBatch() {
	sample := []map[string]interface{}{
		{
			"id":        "1",
			"tenant_id": "sensor-network-a",
			"recipe":    "process",
			"payload":   map[string]interface{}{"temperature": 22.5, "humidity": 65},
		},
		{
			"id":        "2",
			"tenant_id": "sensor-network-a",
			"recipe":    "aggregate",
			"payload":   []interface{}{1.5, 2.3, 4.1, 3.7},
		},
		{
			"id":        "3",
			"tenant_id": "sensor-network-b",
			"recipe":    "echo",
			"payload":   "QUIC is fast!",
		},
	}

	jsonBytes, _ := json.MarshalIndent(sample, "", "  ")
	fmt.Println(string(jsonBytes))
}
