// internal/inference/mock.go
package inference

import (
	"fmt"
)

// MockInference is a mock implementation of InferenceEngine for testing.
// It returns deterministic dummy actions without requiring the ONNX shared library.
type MockInference struct {
	// ActionDim is the number of action dimensions to return per observation
	ActionDim int
	// DefaultAction is the action values returned for each observation
	DefaultAction []float32
	// ShouldError if true, Predict will return an error
	ShouldError bool
	// ErrorMessage is the error message to return when ShouldError is true
	ErrorMessage string
	// CallCount tracks the number of times Predict was called
	CallCount int
}

// NewMock creates a new MockInference with default action [0.1, 0.2, 0.3]
func NewMock() *MockInference {
	return &MockInference{
		ActionDim:     3,
		DefaultAction: []float32{0.1, 0.2, 0.3},
		ShouldError:   false,
	}
}

// NewMockWithAction creates a MockInference with a custom action
func NewMockWithAction(action []float32) *MockInference {
	return &MockInference{
		ActionDim:     len(action),
		DefaultAction: action,
		ShouldError:   false,
	}
}

// Predict returns deterministic dummy actions for each observation in the batch.
// It validates inputs and returns DefaultAction repeated for each observation.
func (m *MockInference) Predict(obsBatch [][]float32, c, h, w int64) ([]float32, error) {
	m.CallCount++

	if m.ShouldError {
		if m.ErrorMessage != "" {
			return nil, fmt.Errorf("%s", m.ErrorMessage)
		}
		return nil, fmt.Errorf("mock inference error")
	}

	batch := len(obsBatch)
	if batch == 0 {
		return nil, fmt.Errorf("empty observation batch")
	}

	// Validate observation sizes
	expectedSize := c * h * w
	for i, obs := range obsBatch {
		if int64(len(obs)) != expectedSize {
			return nil, fmt.Errorf("observation %d has wrong size: got %d, expected %d", i, len(obs), expectedSize)
		}
	}

	// Return deterministic actions for each observation
	result := make([]float32, 0, batch*m.ActionDim)
	for i := 0; i < batch; i++ {
		result = append(result, m.DefaultAction...)
	}

	return result, nil
}

// Close is a no-op for the mock implementation
func (m *MockInference) Close() error {
	return nil
}

// SetError configures the mock to return an error on the next Predict call
func (m *MockInference) SetError(msg string) {
	m.ShouldError = true
	m.ErrorMessage = msg
}

// ClearError clears any configured error
func (m *MockInference) ClearError() {
	m.ShouldError = false
	m.ErrorMessage = ""
}

// Ensure MockInference implements InferenceEngine at compile time
var _ InferenceEngine = (*MockInference)(nil)
