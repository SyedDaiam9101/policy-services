// internal/handler/errors.go
package handler

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcError maps known internal errors to appropriate gRPC status errors
func grpcError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Map specific error patterns to gRPC status codes
	switch {
	case strings.Contains(errMsg, "empty observation batch"):
		return status.Errorf(codes.InvalidArgument, "empty observation batch")

	case strings.Contains(errMsg, "wrong size"):
		return status.Errorf(codes.InvalidArgument, "observation shape mismatch: %v", err)

	case strings.Contains(errMsg, "session is nil"):
		return status.Errorf(codes.FailedPrecondition, "inference engine not initialized")

	case strings.Contains(errMsg, "failed to create input tensor"):
		return status.Errorf(codes.Internal, "tensor creation failed: %v", err)

	case strings.Contains(errMsg, "failed to create output tensor"):
		return status.Errorf(codes.Internal, "tensor creation failed: %v", err)

	case strings.Contains(errMsg, "inference failed"):
		return status.Errorf(codes.Internal, "inference execution failed: %v", err)

	case strings.Contains(errMsg, "failed to initialize"):
		return status.Errorf(codes.FailedPrecondition, "initialization failed: %v", err)

	case strings.Contains(errMsg, "failed to create ONNX session"):
		return status.Errorf(codes.FailedPrecondition, "model loading failed: %v", err)

	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

// invalidArgumentError creates an InvalidArgument gRPC error
func invalidArgumentError(format string, args ...interface{}) error {
	return status.Errorf(codes.InvalidArgument, format, args...)
}

// failedPreconditionError creates a FailedPrecondition gRPC error
func failedPreconditionError(format string, args ...interface{}) error {
	return status.Errorf(codes.FailedPrecondition, format, args...)
}

// internalError creates an Internal gRPC error
func internalError(format string, args ...interface{}) error {
	return status.Errorf(codes.Internal, format, args...)
}
