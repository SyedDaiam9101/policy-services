// internal/handler/handler.go
package handler

import (
	"context"
	"log"
	"time"

	"github.com/SyedDaiam9101/policy-service/internal/cache"
	"github.com/SyedDaiam9101/policy-service/internal/inference"
	"github.com/SyedDaiam9101/policy-service/internal/metrics"
	"github.com/SyedDaiam9101/policy-service/internal/middleware"
	pb "github.com/SyedDaiam9101/policy-service/proto/plannerpb"
)

// Handler implements the PathPlannerServer interface.
// It uses the InferenceEngine interface for flexibility and testability.
type Handler struct {
	pb.UnimplementedPathPlannerServer
	infer inference.InferenceEngine
	cache *cache.Cache
}

// New creates a new Handler with the given inference engine and cache.
// The inference engine must implement the InferenceEngine interface.
func New(infer inference.InferenceEngine, cache *cache.Cache) *Handler {
	return &Handler{
		infer: infer,
		cache: cache,
	}
}

// Plan handles a single planning request by delegating to BatchPlan
func (h *Handler) Plan(ctx context.Context, req *pb.PlanRequest) (*pb.PlanResponse, error) {
	if req == nil {
		return nil, invalidArgumentError("request cannot be nil")
	}

	// Create a batch request with a single element
	batchReq := &pb.BatchPlanRequest{
		Requests: []*pb.PlanRequest{req},
	}

	// Call BatchPlan
	batchResp, err := h.BatchPlan(ctx, batchReq)
	if err != nil {
		return nil, err
	}

	if len(batchResp.Responses) == 0 {
		return nil, internalError("no response from batch plan")
	}

	return batchResp.Responses[0], nil
}

// BatchPlan handles batch planning requests
func (h *Handler) BatchPlan(ctx context.Context, req *pb.BatchPlanRequest) (*pb.BatchPlanResponse, error) {
	start := time.Now()

	// Get request ID for logging
	requestID := middleware.GetRequestID(ctx)
	if requestID == "" {
		requestID = "unknown"
	}

	if req == nil || len(req.Requests) == 0 {
		return nil, invalidArgumentError("batch request cannot be nil or empty")
	}

	if h.infer == nil {
		return nil, failedPreconditionError("inference engine not initialized")
	}

	batchSize := len(req.Requests)

	// Record batch size metric
	metrics.RecordInferenceBatch(batchSize)

	// Extract observations from each request
	var obsBatch [][]float32
	var c, height, w int64

	for i, planReq := range req.Requests {
		if planReq == nil {
			return nil, invalidArgumentError("request %d is nil", i)
		}
		if planReq.Obs == nil {
			return nil, invalidArgumentError("request %d has nil observation", i)
		}

		obs := planReq.Obs

		// Use dimensions from first observation, validate others match
		if i == 0 {
			c = int64(obs.Channels)
			height = int64(obs.Height)
			w = int64(obs.Width)

			// Validate dimensions are positive
			if c <= 0 || height <= 0 || w <= 0 {
				return nil, invalidArgumentError("invalid observation dimensions: channels=%d, height=%d, width=%d", c, height, w)
			}
		} else {
			if int64(obs.Channels) != c || int64(obs.Height) != height || int64(obs.Width) != w {
				return nil, invalidArgumentError(
					"observation %d has mismatched dimensions: got (%d,%d,%d), expected (%d,%d,%d)",
					i, obs.Channels, obs.Height, obs.Width, c, height, w)
			}
		}

		// Validate observation data length
		expectedLen := int(c * height * w)
		if len(obs.Data) != expectedLen {
			return nil, invalidArgumentError(
				"observation %d has wrong data length: got %d, expected %d",
				i, len(obs.Data), expectedLen)
		}

		obsBatch = append(obsBatch, obs.Data)
	}

	// Run inference with timing
	inferStart := time.Now()
	actions, err := h.infer.Predict(obsBatch, c, height, w)
	inferDuration := time.Since(inferStart)
	metrics.RecordInferenceLatency(inferDuration.Seconds())

	if err != nil {
		log.Printf("[%s] Inference error: %v", requestID, err)
		return nil, grpcError(err)
	}

	// Calculate action dimension from output
	actionDim := len(actions) / batchSize
	if actionDim*batchSize != len(actions) {
		return nil, internalError("action output size mismatch: got %d actions for batch %d", len(actions), batchSize)
	}

	// Split actions into per-robot responses
	responses := make([]*pb.PlanResponse, batchSize)
	for i := 0; i < batchSize; i++ {
		startIdx := i * actionDim
		endIdx := startIdx + actionDim

		responses[i] = &pb.PlanResponse{
			Action: actions[startIdx:endIdx],
			Safe:   true, // Placeholder for future confidence logic
		}
	}

	// Log batch metrics
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
	log.Printf("[%s] BatchPlan: batch_size=%d, inference_ms=%.2f, total_ms=%.2f",
		requestID, batchSize, float64(inferDuration.Microseconds())/1000.0, latencyMs)

	return &pb.BatchPlanResponse{
		Responses: responses,
	}, nil
}
