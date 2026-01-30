// internal/inference/inference.go
package inference

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Inference wraps an ONNX runtime session for thread-safe inference.
// It implements the InferenceEngine interface.
type Inference struct {
	mu        sync.Mutex
	session   *ort.DynamicAdvancedSession
	actionDim int64
}

// New creates a new Inference instance by loading the ONNX model from modelPath
func New(modelPath string) (*Inference, error) {
	// Initialize the ONNX runtime environment
	err := ort.InitializeEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX environment: %w", err)
	}

	// Create input/output names - adjust based on your model
	inputNames := []string{"obs"}
	outputNames := []string{"action"}

	// Create a dynamic session that supports variable batch sizes
	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		inputNames,
		outputNames,
		nil, // Use default session options
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	return &Inference{
		session:   session,
		actionDim: 2, // Default action dimension, adjust as needed
	}, nil
}

// Predict runs batch inference on observations.
// obsBatch: slice of flattened observations, each of length C*H*W
// c, h, w: channel, height, width dimensions
// Returns flattened actions of length batch * actionDim
func (inf *Inference) Predict(obsBatch [][]float32, c, h, w int64) ([]float32, error) {
	inf.mu.Lock()
	defer inf.mu.Unlock()

	if inf.session == nil {
		return nil, fmt.Errorf("inference session is nil")
	}

	batch := int64(len(obsBatch))
	if batch == 0 {
		return nil, fmt.Errorf("empty observation batch")
	}

	// Calculate expected observation size
	obsSize := c * h * w

	// Pack batch into a single tensor [batch, C, H, W]
	tensorData := make([]float32, 0, batch*obsSize)
	for i, obs := range obsBatch {
		if int64(len(obs)) != obsSize {
			return nil, fmt.Errorf("observation %d has wrong size: got %d, expected %d", i, len(obs), obsSize)
		}
		tensorData = append(tensorData, obs...)
	}

	// Create input tensor with shape [batch, C, H, W]
	inputShape := ort.NewShape(batch, c, h, w)
	inputTensor, err := ort.NewTensor(inputShape, tensorData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	// Create output tensor with shape [batch, actionDim]
	outputShape := ort.NewShape(batch, inf.actionDim)
	outputData := make([]float32, batch*inf.actionDim)
	outputTensor, err := ort.NewTensor(outputShape, outputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Run inference
	err = inf.session.Run(
		[]ort.ArbitraryTensor{inputTensor},
		[]ort.ArbitraryTensor{outputTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Return the output data
	return outputTensor.GetData(), nil
}

// Close releases the ONNX session resources
func (inf *Inference) Close() error {
	inf.mu.Lock()
	defer inf.mu.Unlock()

	if inf.session != nil {
		err := inf.session.Destroy()
		inf.session = nil
		if err != nil {
			return fmt.Errorf("failed to destroy session: %w", err)
		}
	}

	return ort.DestroyEnvironment()
}

// SetActionDim sets the action dimension for the model
func (inf *Inference) SetActionDim(dim int64) {
	inf.mu.Lock()
	defer inf.mu.Unlock()
	inf.actionDim = dim
}

// Ensure Inference implements InferenceEngine at compile time
var _ InferenceEngine = (*Inference)(nil)
