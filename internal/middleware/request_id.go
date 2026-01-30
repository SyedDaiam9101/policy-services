// internal/middleware/request_id.go
package middleware

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// RequestIDHeader is the metadata key for the request ID
	RequestIDHeader = "x-request-id"
)

// requestIDKey is the context key for storing the request ID
type requestIDKey struct{}

// UnaryRequestIDInterceptor extracts x-request-id from incoming metadata or generates
// a new UUID if not present. It injects the request ID into the context and adds it
// to outgoing metadata.
func UnaryRequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Try to extract request ID from incoming metadata
		requestID := extractRequestID(ctx)

		// Generate a new UUID if not present
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add request ID to context
		ctx = context.WithValue(ctx, requestIDKey{}, requestID)

		// Add request ID to outgoing metadata (response headers)
		header := metadata.Pairs(RequestIDHeader, requestID)
		if err := grpc.SetHeader(ctx, header); err != nil {
			// Log but don't fail the request
			// The header might already be sent in streaming scenarios
		}

		// Call the handler
		return handler(ctx, req)
	}
}

// extractRequestID extracts the request ID from incoming metadata
func extractRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get(RequestIDHeader)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}
