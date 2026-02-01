package relayer

import (
	"testing"
	"time"
)

func TestError_Error(t *testing.T) {
	err := &Error{
		Code:    "TEST_ERROR",
		Message: "This is a test error",
	}

	expected := "[TEST_ERROR] This is a test error"
	if got := err.Error(); got != expected {
		t.Errorf("Error.Error() = %q, want %q", got, expected)
	}
}

func TestFilterSuccess(t *testing.T) {
	responses := []Response{
		{ID: "1", Status: 200, Data: "success"},
		{ID: "2", Status: 404, Error: &Error{Code: "NOT_FOUND", Message: "not found"}},
		{ID: "3", Status: 201, Data: "created"},
		{ID: "4", Status: 500, Error: &Error{Code: "ERROR", Message: "internal error"}},
		{ID: "5", Status: 204, Data: nil},
	}

	successes := FilterSuccess(responses)

	expectedCount := 3
	if len(successes) != expectedCount {
		t.Errorf("FilterSuccess() returned %d responses, want %d", len(successes), expectedCount)
	}

	// Verify all success responses have 2xx status codes
	for _, resp := range successes {
		if resp.Status < 200 || resp.Status >= 300 {
			t.Errorf("FilterSuccess() returned response with status %d, want 2xx", resp.Status)
		}
	}

	// Verify correct IDs
	expectedIDs := map[string]bool{"1": true, "3": true, "5": true}
	for _, resp := range successes {
		if !expectedIDs[resp.ID] {
			t.Errorf("FilterSuccess() returned unexpected ID %s", resp.ID)
		}
	}
}

func TestFilterByStatus(t *testing.T) {
	responses := []Response{
		{ID: "1", Status: 200},
		{ID: "2", Status: 404},
		{ID: "3", Status: 200},
		{ID: "4", Status: 500},
	}

	filtered := FilterByStatus(responses, 200)

	if len(filtered) != 2 {
		t.Errorf("FilterByStatus(200) returned %d responses, want 2", len(filtered))
	}

	for _, resp := range filtered {
		if resp.Status != 200 {
			t.Errorf("FilterByStatus(200) returned response with status %d", resp.Status)
		}
	}
}

func TestFilterByTenant(t *testing.T) {
	responses := []Response{
		{ID: "1", TenantID: "tenant-a", Status: 200},
		{ID: "2", TenantID: "tenant-b", Status: 200},
		{ID: "3", TenantID: "tenant-a", Status: 404},
		{ID: "4", TenantID: "tenant-c", Status: 200},
	}

	filtered := FilterByTenant(responses, "tenant-a")

	if len(filtered) != 2 {
		t.Errorf("FilterByTenant('tenant-a') returned %d responses, want 2", len(filtered))
	}

	for _, resp := range filtered {
		if resp.TenantID != "tenant-a" {
			t.Errorf("FilterByTenant('tenant-a') returned response with tenant %s", resp.TenantID)
		}
	}
}

func TestFilterSuccess_EmptyInput(t *testing.T) {
	responses := []Response{}
	successes := FilterSuccess(responses)

	if len(successes) != 0 {
		t.Errorf("FilterSuccess([]) = %d responses, want 0", len(successes))
	}
}

func TestFilterByStatus_NoMatches(t *testing.T) {
	responses := []Response{
		{ID: "1", Status: 200},
		{ID: "2", Status: 404},
	}

	filtered := FilterByStatus(responses, 500)

	if len(filtered) != 0 {
		t.Errorf("FilterByStatus(500) = %d responses, want 0", len(filtered))
	}
}

func TestFilterByTenant_NoMatches(t *testing.T) {
	responses := []Response{
		{ID: "1", TenantID: "tenant-a"},
		{ID: "2", TenantID: "tenant-b"},
	}

	filtered := FilterByTenant(responses, "tenant-c")

	if len(filtered) != 0 {
		t.Errorf("FilterByTenant('tenant-c') = %d responses, want 0", len(filtered))
	}
}

func TestResponse_Duration(t *testing.T) {
	resp := Response{
		ID:       "test",
		Status:   200,
		Duration: 150 * time.Millisecond,
	}

	if resp.Duration != 150*time.Millisecond {
		t.Errorf("Response.Duration = %v, want %v", resp.Duration, 150*time.Millisecond)
	}
}

func TestError_WithDetails(t *testing.T) {
	err := &Error{
		Code:    "VALIDATION_ERROR",
		Message: "Invalid input",
		Details: map[string]interface{}{
			"field": "email",
			"issue": "invalid format",
		},
	}

	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("Error.Code = %q, want %q", err.Code, "VALIDATION_ERROR")
	}

	if err.Details["field"] != "email" {
		t.Errorf("Error.Details['field'] = %v, want %v", err.Details["field"], "email")
	}
}
