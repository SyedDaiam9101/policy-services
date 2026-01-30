// internal/middleware/metrics.go
package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/SyedDaiam9101/policy-service/internal/metrics"
)

// UnaryMetricsInterceptor records Prometheus histogram metrics for gRPC unary calls.
// It measures the duration of each call and records it with method and status code labels.
func UnaryMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call the handler
		resp, err := handler(ctx, req)

		// Record the duration
		duration := time.Since(start).Seconds()

		// Extract status code
		code := "OK"
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code().String()
			} else {
				code = "Unknown"
			}
		}

		// Record the metric
		metrics.RecordGRPCLatency(info.FullMethod, code, duration)

		return resp, err
	}
}
