package relayer

import (
	"context"
	"testing"
)

func TestTenantID(t *testing.T) {
	ctx := context.Background()

	// Test missing tenant ID
	if _, ok := TenantID(ctx); ok {
		t.Error("TenantID() returned ok=true for context without tenant ID")
	}

	// Test with tenant ID
	ctx = WithTenantID(ctx, "tenant-123")
	tenantID, ok := TenantID(ctx)
	if !ok {
		t.Error("TenantID() returned ok=false after WithTenantID()")
	}
	if tenantID != "tenant-123" {
		t.Errorf("TenantID() = %q, want %q", tenantID, "tenant-123")
	}
}

func TestRequestID(t *testing.T) {
	ctx := context.Background()

	// Test missing request ID
	if _, ok := RequestID(ctx); ok {
		t.Error("RequestID() returned ok=true for context without request ID")
	}

	// Test with request ID
	ctx = WithRequestID(ctx, "req-456")
	requestID, ok := RequestID(ctx)
	if !ok {
		t.Error("RequestID() returned ok=false after WithRequestID()")
	}
	if requestID != "req-456" {
		t.Errorf("RequestID() = %q, want %q", requestID, "req-456")
	}
}

func TestRecipeName(t *testing.T) {
	ctx := context.Background()

	// Test missing recipe name
	if _, ok := RecipeName(ctx); ok {
		t.Error("RecipeName() returned ok=true for context without recipe name")
	}

	// Test with recipe name
	ctx = WithRecipeName(ctx, "my-recipe")
	recipeName, ok := RecipeName(ctx)
	if !ok {
		t.Error("RecipeName() returned ok=false after WithRecipeName()")
	}
	if recipeName != "my-recipe" {
		t.Errorf("RecipeName() = %q, want %q", recipeName, "my-recipe")
	}
}

func TestContextKeysDoNotCollide(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tenant-123")
	ctx = WithRequestID(ctx, "req-456")
	ctx = WithRecipeName(ctx, "recipe-789")

	// Verify all values are independently retrievable
	tenantID, ok := TenantID(ctx)
	if !ok || tenantID != "tenant-123" {
		t.Errorf("TenantID() = %q, %v; want %q, true", tenantID, ok, "tenant-123")
	}

	requestID, ok := RequestID(ctx)
	if !ok || requestID != "req-456" {
		t.Errorf("RequestID() = %q, %v; want %q, true", requestID, ok, "req-456")
	}

	recipeName, ok := RecipeName(ctx)
	if !ok || recipeName != "recipe-789" {
		t.Errorf("RecipeName() = %q, %v; want %q, true", recipeName, ok, "recipe-789")
	}
}

func TestContextKeysTypeSafety(t *testing.T) {
	ctx := context.Background()

	// Test that using wrong type doesn't panic
	ctx = context.WithValue(ctx, "tenant_id", "should-not-work")

	// Should not find the tenant ID because key type is wrong
	if _, ok := TenantID(ctx); ok {
		t.Error("TenantID() found value with wrong key type")
	}
}

func TestWithTenantID_Overwrite(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tenant-1")
	ctx = WithTenantID(ctx, "tenant-2")

	tenantID, ok := TenantID(ctx)
	if !ok {
		t.Error("TenantID() returned ok=false")
	}
	if tenantID != "tenant-2" {
		t.Errorf("TenantID() = %q, want %q (should be overwritten)", tenantID, "tenant-2")
	}
}

func TestContextPreservesParentValues(t *testing.T) {
	parent := context.Background()
	parent = WithTenantID(parent, "tenant-parent")

	child := WithRequestID(parent, "req-child")

	// Child should have both parent's tenant ID and its own request ID
	tenantID, ok := TenantID(child)
	if !ok || tenantID != "tenant-parent" {
		t.Errorf("Child context lost parent's tenant ID: got %q, %v", tenantID, ok)
	}

	requestID, ok := RequestID(child)
	if !ok || requestID != "req-child" {
		t.Errorf("Child context missing request ID: got %q, %v", requestID, ok)
	}
}
