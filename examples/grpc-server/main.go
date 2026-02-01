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
 * gRPC Server Example for Relayer Batch Library
 *
 * This example demonstrates how to integrate the relayer library with gRPC.
 *
 * To use this example:
 * 1. Install protoc: https://grpc.io/docs/protoc-installation/
 * 2. Install Go gRPC tools:
 *    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
 *    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
 * 3. Generate code:
 *    protoc --go_out=. --go-grpc_out=. batch.proto
 * 4. Install dependencies:
 *    go get google.golang.org/grpc
 *    go get google.golang.org/protobuf
 * 5. Run:
 *    go run main.go batch.pb.go batch_grpc.pb.go
 *
 * Note: This file provides the implementation logic. You'll need to generate
 * the protobuf code before this can be compiled and run.
 */

// BatchServer implements the gRPC BatchService
type BatchServer struct {
	// UnimplementedBatchServiceServer // Uncomment after protoc generation
	orchestrator *relayer.Orchestrator
}

func NewBatchServer() *BatchServer {
	orch := relayer.New(
		relayer.WithTimeout(10 * time.Second),
		relayer.WithMaxConcurrency(100),
	)

	setupRecipes(orch)

	return &BatchServer{
		orchestrator: orch,
	}
}

// ExecuteBatch implements the BatchService.ExecuteBatch RPC method
// func (s *BatchServer) ExecuteBatch(ctx context.Context, req *BatchRequest) (*BatchResponse, error) {
func ExecuteBatchExample(ctx context.Context, orch *relayer.Orchestrator) {
	// Example implementation (uncomment and adapt after protoc generation)

	/*
	// Convert proto requests to relayer SubRequests
	batch := make([]relayer.SubRequest, len(req.Requests))
	for i, protoReq := range req.Requests {
		// Parse JSON payload
		var payload interface{}
		if err := json.Unmarshal([]byte(protoReq.PayloadJson), &payload); err != nil {
			log.Printf("Failed to parse payload for request %s: %v", protoReq.Id, err)
			payload = protoReq.PayloadJson // Fallback to string
		}

		batch[i] = relayer.SubRequest{
			ID:       protoReq.Id,
			TenantID: protoReq.TenantId,
			Recipe:   protoReq.Recipe,
			Payload:  payload,
		}
	}

	// Execute batch
	results := s.orchestrator.ExecuteBatch(ctx, batch)

	// Convert results to proto responses
	protoResponses := make([]*SubResponseProto, len(results))
	successes := 0

	for i, resp := range results {
		// Serialize data to JSON
		var dataJSON string
		if resp.Data != nil {
			jsonBytes, err := json.Marshal(resp.Data)
			if err != nil {
				log.Printf("Failed to marshal response data: %v", err)
			} else {
				dataJSON = string(jsonBytes)
			}
		}

		// Convert error if present
		var protoError *ErrorProto
		if resp.Error != nil {
			protoError = &ErrorProto{
				Code:    resp.Error.Code,
				Message: resp.Error.Message,
			}
		}

		protoResponses[i] = &SubResponseProto{
			Id:         resp.ID,
			Status:     int32(resp.Status),
			DataJson:   dataJSON,
			Error:      protoError,
			DurationMs: resp.Duration.Milliseconds(),
			TenantId:   resp.TenantID,
		}

		if resp.Status >= 200 && resp.Status < 300 {
			successes++
		}
	}

	return &BatchResponse{
		Responses: protoResponses,
		Summary: &BatchSummary{
			Total:     int32(len(results)),
			Successes: int32(successes),
			Failures:  int32(len(results) - successes),
		},
	}, nil
	*/
}

// StreamBatch implements the BatchService.StreamBatch RPC method
// func (s *BatchServer) StreamBatch(req *BatchRequest, stream BatchService_StreamBatchServer) error {
func StreamBatchExample(orch *relayer.Orchestrator) {
	// Example implementation for streaming responses
	/*
	// Convert and execute as above
	batch := convertProtoToBatch(req)
	results := s.orchestrator.ExecuteBatch(context.Background(), batch)

	// Stream each result as it completes
	for _, resp := range results {
		protoResp := convertToProtoResponse(resp)
		if err := stream.Send(protoResp); err != nil {
			return err
		}
	}

	return nil
	*/
}

func setupRecipes(orch *relayer.Orchestrator) {
	// Echo recipe
	orch.RegisterRecipe("echo", func(ctx context.Context, payload interface{}) (interface{}, error) {
		return payload, nil
	})

	// Greeting recipe
	orch.RegisterRecipe("greet", func(ctx context.Context, payload interface{}) (interface{}, error) {
		tenantID, _ := relayer.TenantID(ctx)
		name, ok := payload.(string)
		if !ok {
			return nil, fmt.Errorf("payload must be string")
		}
		return fmt.Sprintf("Hello %s from %s!", name, tenantID), nil
	})

	// JSON processing recipe
	orch.RegisterRecipe("process-json", func(ctx context.Context, payload interface{}) (interface{}, error) {
		data, ok := payload.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("payload must be JSON object")
		}

		// Example processing
		return map[string]interface{}{
			"processed": true,
			"original":  data,
			"timestamp": time.Now().Unix(),
		}, nil
	})

	log.Println("gRPC server recipes registered")
}

func main() {
	log.Println("gRPC Server Example")
	log.Println("===================")
	log.Println()
	log.Println("This example demonstrates gRPC integration with the batching library.")
	log.Println()
	log.Println("To use this example, you need to:")
	log.Println("1. Generate protobuf code from batch.proto:")
	log.Println("   protoc --go_out=. --go-grpc_out=. batch.proto")
	log.Println()
	log.Println("2. Install gRPC dependencies:")
	log.Println("   go get google.golang.org/grpc")
	log.Println("   go get google.golang.org/protobuf")
	log.Println()
	log.Println("3. Uncomment the server code above and run:")
	log.Println("   go run main.go batch.pb.go batch_grpc.pb.go")
	log.Println()
	log.Println("Example client call using grpcurl:")
	log.Println("   grpcurl -plaintext -d @request.json localhost:50051 batch.BatchService/ExecuteBatch")
	log.Println()
	log.Println("Sample request.json:")
	printSampleRequest()

	// Uncomment after protoc generation:
	/*
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	RegisterBatchServiceServer(grpcServer, NewBatchServer())

	log.Println("gRPC server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	*/
}

func printSampleRequest() {
	sample := map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"id":           "1",
				"tenant_id":    "company-a",
				"recipe":       "greet",
				"payload_json": `"Alice"`,
			},
			{
				"id":           "2",
				"tenant_id":    "company-b",
				"recipe":       "echo",
				"payload_json": `{"message": "Hello gRPC"}`,
			},
		},
	}

	jsonBytes, _ := json.MarshalIndent(sample, "", "  ")
	fmt.Println(string(jsonBytes))
}
