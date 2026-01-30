// internal/inference/inference_test.go
package inference

import (
	"os"
	"testing"
)

func TestMockInference_Predict(t *testing.T) {
	mock := NewMock()

	// Create test observation batch
	obsBatch := [][]float32{
		{0.1, 0.2, 0.3, 0.4}, // C=1, H=2, W=2
		{0.5, 0.6, 0.7, 0.8},
	}

	actions, err := mock.Predict(obsBatch, 1, 2, 2)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	// Should return 3 actions per observation (default mock action)
	expectedLen := 2 * 3 // 2 observations * 3 actions each
	if len(actions) != expectedLen {
		t.Errorf("Expected %d actions, got %d", expectedLen, len(actions))
	}

	// Verify the action values
	expectedAction := []float32{0.1, 0.2, 0.3}
	for i := 0; i < 2; i++ {
		for j := 0; j < 3; j++ {
			idx := i*3 + j
			if actions[idx] != expectedAction[j] {
				t.Errorf("Action[%d] = %f, expected %f", idx, actions[idx], expectedAction[j])
			}
		}
	}

	// Verify call count
	if mock.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", mock.CallCount)
	}
}

func TestMockInference_PredictError(t *testing.T) {
	mock := NewMock()
	mock.SetError("test error")

	obsBatch := [][]float32{{0.1, 0.2, 0.3, 0.4}}
	_, err := mock.Predict(obsBatch, 1, 2, 2)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%s'", err.Error())
	}
}

func TestMockInference_EmptyBatch(t *testing.T) {
	mock := NewMock()
	_, err := mock.Predict([][]float32{}, 1, 2, 2)
	if err == nil {
		t.Fatal("Expected error for empty batch")
	}
}

func TestMockInference_WrongObservationSize(t *testing.T) {
	mock := NewMock()
	obsBatch := [][]float32{
		{0.1, 0.2}, // Wrong size: expected 4 (1*2*2)
	}

	_, err := mock.Predict(obsBatch, 1, 2, 2)
	if err == nil {
		t.Fatal("Expected error for wrong observation size")
	}
}

func TestMockInference_CustomAction(t *testing.T) {
	customAction := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
	mock := NewMockWithAction(customAction)

	obsBatch := [][]float32{{0.1, 0.2, 0.3, 0.4}}
	actions, err := mock.Predict(obsBatch, 1, 2, 2)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(actions) != len(customAction) {
		t.Errorf("Expected %d actions, got %d", len(customAction), len(actions))
	}

	for i, v := range customAction {
		if actions[i] != v {
			t.Errorf("Action[%d] = %f, expected %f", i, actions[i], v)
		}
	}
}

func TestRealInference_WithModel(t *testing.T) {
	// Skip if ONNX model or library is not available
	modelPath := "testdata/dummy.onnx"
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skip("Skipping real inference test: testdata/dummy.onnx not found")
	}

	// Try to create inference - will fail if ONNX library not installed
	infer, err := New(modelPath)
	if err != nil {
		t.Skipf("Skipping real inference test: %v", err)
	}
	defer infer.Close()

	// Set action dimension (adjust based on your dummy model)
	infer.SetActionDim(2)

	// Test with a single observation
	obsBatch := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
	}

	actions, err := infer.Predict(obsBatch, 1, 2, 2)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	// Verify output length matches actionDim
	expectedLen := 1 * 2 // batch=1, actionDim=2
	if len(actions) != expectedLen {
		t.Errorf("Expected %d actions, got %d", expectedLen, len(actions))
	}
}
