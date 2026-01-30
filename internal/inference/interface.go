// internal/inference/interface.go
package inference

// InferenceEngine defines the interface for running batch inference.
// This abstraction allows for easy mocking in tests and swapping implementations.
type InferenceEngine interface {
	// Predict runs a batch of observations and returns the flattened actions.
	// obsBatch: slice of flattened observations, each of length C*H*W
	// c, h, w: channel, height, width dimensions
	// Returns flattened actions of length batch * actionDim
	Predict(obsBatch [][]float32, c, h, w int64) ([]float32, error)

	// Close releases any resources held by the inference engine.
	Close() error
}
