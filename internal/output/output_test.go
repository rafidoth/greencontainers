package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"greencontainers/internal/profiler"
)

func TestSerialization(t *testing.T) {
	tempDir := t.TempDir()

	res := profiler.ExperimentResult{
		ExperimentID: "test-exp",
		Stages: []profiler.StageResult{
			{
				ExperimentID: "test-exp",
				TrialID:      "test-trial-1",
				Stage:        profiler.StageIdle,
				StartedAt:    time.Now(),
				FinishedAt:   time.Now().Add(time.Second),
				Duration:     time.Second,
				RawEnergyJoules: 10.5,
				Success: true,
			},
		},
	}

	csvPath := filepath.Join(tempDir, "test.csv")
	err := WriteCSV(csvPath, res)
	if err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Errorf("CSV file was not created")
	}

	jsonPath := filepath.Join(tempDir, "test.json")
	err = WriteJSON(jsonPath, res)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("JSON file was not created")
	}

	// Basic JSON verification
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read JSON: %v", err)
	}

	var parsed profiler.ExperimentResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed.ExperimentID != "test-exp" {
		t.Errorf("Expected ExperimentID 'test-exp', got '%s'", parsed.ExperimentID)
	}
}
