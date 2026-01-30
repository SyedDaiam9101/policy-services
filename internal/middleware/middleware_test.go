// internal/middleware/middleware_test.go
package middleware

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryRequestIDInterceptor_GeneratesID(t *testing.T) {
	interceptor := UnaryRequestIDInterceptor()

	// Create a mock handler that captures the context
	var capturedCtx context.Context
	mockHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "response", nil
	}

	// Call with empty context (no incoming metadata)
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, mockHandler)
	if err != nil {
		t.Fatalf("Interceptor failed: %v", err)
	}

	// Verify request ID was generated and added to context
	requestID := GetRequestID(capturedCtx)
	if requestID == "" {
		t.Error("Expected request ID to be generated, got empty string")
	}

	// Verify it looks like a UUID (36 chars with dashes)
	if len(requestID) != 36 {
		t.Errorf("Expected UUID format (36 chars), got %d chars: %s", len(requestID), requestID)
	}
}

func TestUnaryRequestIDInterceptor_PreservesExistingID(t *testing.T) {
	interceptor := UnaryRequestIDInterceptor()

	existingID := "test-request-id-12345"

	var capturedCtx context.Context
	mockHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "response", nil
	}

	// Create context with incoming metadata containing request ID
	md := metadata.Pairs(RequestIDHeader, existingID)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(ctx, nil, info, mockHandler)
	if err != nil {
		t.Fatalf("Interceptor failed: %v", err)
	}

	// Verify the existing request ID was preserved
	requestID := GetRequestID(capturedCtx)
	if requestID != existingID {
		t.Errorf("Expected request ID %s, got %s", existingID, requestID)
	}
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	requestID := GetRequestID(ctx)
	if requestID != "" {
		t.Errorf("Expected empty request ID from empty context, got %s", requestID)
	}
}
