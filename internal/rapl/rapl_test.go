package rapl

import (
	"testing"
)

func TestCalculateDelta(t *testing.T) {
	tests := []struct {
		name     string
		before   Reading
		after    Reading
		expected uint64
	}{
		{
			name: "normal increase",
			before: Reading{Domains: map[string]DomainData{
				"domain0": {ID: "domain0", EnergyMicroJoules: 100, MaxEnergyRangeUJ: 1000},
			}},
			after: Reading{Domains: map[string]DomainData{
				"domain0": {ID: "domain0", EnergyMicroJoules: 300, MaxEnergyRangeUJ: 1000},
			}},
			expected: 200,
		},
		{
			name: "wraparound",
			before: Reading{Domains: map[string]DomainData{
				"domain0": {ID: "domain0", EnergyMicroJoules: 900, MaxEnergyRangeUJ: 1000},
			}},
			after: Reading{Domains: map[string]DomainData{
				"domain0": {ID: "domain0", EnergyMicroJoules: 100, MaxEnergyRangeUJ: 1000},
			}},
			expected: 200, // (1000 - 900) + 100
		},
		{
			name: "multiple domains",
			before: Reading{Domains: map[string]DomainData{
				"domain0": {ID: "domain0", EnergyMicroJoules: 100, MaxEnergyRangeUJ: 1000},
				"domain1": {ID: "domain1", EnergyMicroJoules: 800, MaxEnergyRangeUJ: 1000},
			}},
			after: Reading{Domains: map[string]DomainData{
				"domain0": {ID: "domain0", EnergyMicroJoules: 200, MaxEnergyRangeUJ: 1000},
				"domain1": {ID: "domain1", EnergyMicroJoules: 100, MaxEnergyRangeUJ: 1000}, // wrap
			}},
			expected: 100 + 300, // 400
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelta(tt.before, tt.after)
			if got != tt.expected {
				t.Errorf("CalculateDelta() = %v, want %v", got, tt.expected)
			}
		})
	}
}
