// internal/handler/handler_test.go
package handler

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/SyedDaiam9101/policy-service/internal/inference"
	"github.com/SyedDaiam9101/policy-service/internal/middleware"
	pb "github.com/SyedDaiam9101/policy-service/proto/plannerpb"
)

func TestPlanWithNilInference(t *testing.T) {
	h := New(nil, nil)

	req := &pb.PlanRequest{
		RobotId: 1,
		Obs: &pb.Observation{
			Data:     []float32{0.1, 0.2, 0.3, 0.4},
			Channels: 1,
			Height:   2,
			Width:    2,
		},
	}

	_, err := h.Plan(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error when inference is nil, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.FailedPrecondition {
		t.Errorf("Expected FailedPrecondition, got: %v", st.Code())
	}
}

func TestPlanWithNilRequest(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	_, err := h.Plan(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil request, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got: %v", st.Code())
	}
}

func TestPlanWithMockInference(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	req := &pb.PlanRequest{
		RobotId: 1,
		Obs: &pb.Observation{
			Data:     []float32{0.1, 0.2, 0.3, 0.4},
			Channels: 1,
			Height:   2,
			Width:    2,
		},
	}

	resp, err := h.Plan(context.Background(), req)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Mock returns [0.1, 0.2, 0.3]
	expectedActions := []float32{0.1, 0.2, 0.3}
	if len(resp.Action) != len(expectedActions) {
		t.Errorf("Expected %d actions, got %d", len(expectedActions), len(resp.Action))
	}

	for i, v := range expectedActions {
		if resp.Action[i] != v {
			t.Errorf("Action[%d] = %f, expected %f", i, resp.Action[i], v)
		}
	}

	if !resp.Safe {
		t.Error("Expected Safe=true")
	}

	// Verify mock was called
	if mock.CallCount != 1 {
		t.Errorf("Expected mock.CallCount=1, got %d", mock.CallCount)
	}
}

func TestBatchPlanWithMockInference(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{
			{
				RobotId: 1,
				Obs: &pb.Observation{
					Data:     []float32{0.1, 0.2, 0.3, 0.4},
					Channels: 1,
					Height:   2,
					Width:    2,
				},
			},
			{
				RobotId: 2,
				Obs: &pb.Observation{
					Data:     []float32{0.5, 0.6, 0.7, 0.8},
					Channels: 1,
					Height:   2,
					Width:    2,
				},
			},
		},
	}

	resp, err := h.BatchPlan(context.Background(), req)
	if err != nil {
		t.Fatalf("BatchPlan failed: %v", err)
	}

	if len(resp.Responses) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(resp.Responses))
	}

	// Verify mock was called once for the batch
	if mock.CallCount != 1 {
		t.Errorf("Expected mock.CallCount=1, got %d", mock.CallCount)
	}
}

func TestBatchPlanWithEmptyRequests(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{},
	}

	_, err := h.BatchPlan(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for empty batch request, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got: %v", st.Code())
	}
}

func TestBatchPlanWithNilObservation(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{
			{RobotId: 1, Obs: nil},
		},
	}

	_, err := h.BatchPlan(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for nil observation, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got: %v", st.Code())
	}
}

func TestBatchPlanWithMismatchedDimensions(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{
			{
				RobotId: 1,
				Obs: &pb.Observation{
					Data:     []float32{0.1, 0.2, 0.3, 0.4},
					Channels: 1,
					Height:   2,
					Width:    2,
				},
			},
			{
				RobotId: 2,
				Obs: &pb.Observation{
					Data:     []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
					Channels: 2, // Different channels!
					Height:   2,
					Width:    2,
				},
			},
		},
	}

	_, err := h.BatchPlan(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for mismatched dimensions, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got: %v", st.Code())
	}

	if !strings.Contains(st.Message(), "mismatched dimensions") {
		t.Errorf("Expected error message about mismatched dimensions, got: %s", st.Message())
	}
}

func TestBatchPlanWithInvalidDataLength(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{
			{
				RobotId: 1,
				Obs: &pb.Observation{
					Data:     []float32{0.1, 0.2}, // Too short!
					Channels: 1,
					Height:   2,
					Width:    2,
				},
			},
		},
	}

	_, err := h.BatchPlan(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for invalid data length, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument, got: %v", st.Code())
	}
}

func TestBatchPlanWithRequestID(t *testing.T) {
	mock := inference.NewMock()
	h := New(mock, nil)

	// Simulate request with request ID in context
	testRequestID := "test-request-id-123"
	md := metadata.Pairs(middleware.RequestIDHeader, testRequestID)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Process through request ID interceptor
	interceptor := middleware.UnaryRequestIDInterceptor()
	var capturedCtx context.Context

	// Wrap the handler call
	wrappedHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return h.BatchPlan(ctx, req.(*pb.BatchPlanRequest))
	}

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{
			{
				RobotId: 1,
				Obs: &pb.Observation{
					Data:     []float32{0.1, 0.2, 0.3, 0.4},
					Channels: 1,
					Height:   2,
					Width:    2,
				},
			},
		},
	}

	_, err := interceptor(ctx, req, nil, wrappedHandler)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// Verify request ID was in context
	extractedID := middleware.GetRequestID(capturedCtx)
	if extractedID != testRequestID {
		t.Errorf("Expected request ID %s, got %s", testRequestID, extractedID)
	}
}

func TestBatchPlanWithInferenceError(t *testing.T) {
	mock := inference.NewMock()
	mock.SetError("model execution failed")
	h := New(mock, nil)

	req := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{
			{
				RobotId: 1,
				Obs: &pb.Observation{
					Data:     []float32{0.1, 0.2, 0.3, 0.4},
					Channels: 1,
					Height:   2,
					Width:    2,
				},
			},
		},
	}

	_, err := h.BatchPlan(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error from inference, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	// Should be mapped to Internal error
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal error code, got: %v", st.Code())
	}
}
