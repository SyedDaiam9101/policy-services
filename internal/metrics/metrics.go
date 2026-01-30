// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// GRPCServerHandlingSeconds is a histogram for gRPC server request latencies
	GRPCServerHandlingSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "code"},
	)

	// InferenceBatchSize is a histogram for tracking inference batch sizes
	InferenceBatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "inference_batch_size",
			Help:    "Histogram of batch sizes for inference requests.",
			Buckets: []float64{1, 2, 4, 8, 16, 32, 64, 128, 256},
		},
	)

	// InferenceLatencySeconds is a histogram for inference-only latency
	InferenceLatencySeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "inference_latency_seconds",
			Help:    "Histogram of inference latency (seconds) excluding gRPC overhead.",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// HealthStatus is a gauge indicating the health status of the service
	HealthStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "health_status",
			Help: "Health status of the service (1 = healthy, 0 = unhealthy).",
		},
	)
)

// RecordGRPCLatency records the latency of a gRPC method call
func RecordGRPCLatency(method, code string, seconds float64) {
	GRPCServerHandlingSeconds.WithLabelValues(method, code).Observe(seconds)
}

// RecordInferenceBatch records the batch size for an inference request
func RecordInferenceBatch(size int) {
	InferenceBatchSize.Observe(float64(size))
}

// RecordInferenceLatency records the latency of an inference call
func RecordInferenceLatency(seconds float64) {
	InferenceLatencySeconds.Observe(seconds)
}

// SetHealthy sets the health status to healthy
func SetHealthy() {
	HealthStatus.Set(1)
}

// SetUnhealthy sets the health status to unhealthy
func SetUnhealthy() {
	HealthStatus.Set(0)
}
