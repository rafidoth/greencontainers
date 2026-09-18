package baseline

import (
	"context"
	"testing"
	"time"

	"greencontainers/internal/rapl"
)

type mockRAPL struct {
	readCount int
	baseTime  time.Time
}

func (m *mockRAPL) Read() (rapl.Reading, error) {
	if m.baseTime.IsZero() {
		m.baseTime = time.Now()
	}
	m.readCount++
	
	domains := map[string]rapl.DomainData{
		"domain0": {ID: "domain0", EnergyMicroJoules: uint64(m.readCount * 1000000), MaxEnergyRangeUJ: 10000000},
	}
	
	// Increment timestamp by 1s for each read
	return rapl.Reading{
		Timestamp: m.baseTime.Add(time.Duration(m.readCount) * time.Second),
		Domains:   domains,
	}, nil
}

func TestBaselineMeasure(t *testing.T) {
	ctx := context.Background()
	r := &mockRAPL{}
	
	// Keep duration short for testing
	res, err := Measure(ctx, 100*time.Millisecond, r, 50*time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Measure failed: %v", err)
	}

	if res.EnergyJoules != 1.0 { // (2000000 - 1000000) uJ = 1 J
		t.Errorf("Expected 1.0 J, got %v", res.EnergyJoules)
	}
	
	if res.PowerWatts != 1.0 { // 1 J / 1 s
		t.Errorf("Expected 1.0 W, got %v", res.PowerWatts)
	}
}
