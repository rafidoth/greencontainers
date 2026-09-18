//go:build linux

package rapl

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DomainData struct {
	ID               string
	EnergyMicroJoules uint64
	MaxEnergyRangeUJ  uint64
}

type Reading struct {
	Timestamp time.Time
	Domains   map[string]DomainData
}

type Reader interface {
	Read() (Reading, error)
}

type raplReader struct {
	domainPaths map[string]string
	maxEnergies map[string]uint64
}

func NewReader() (Reader, error) {
	basePath := "/sys/class/powercap"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read powercap dir (is RAPL supported?): %w", err)
	}

	domainPaths := make(map[string]string)
	maxEnergies := make(map[string]uint64)

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "intel-rapl:") && !strings.Contains(entry.Name(), ":0:") {
			domainPath := filepath.Join(basePath, entry.Name())
			
			maxEnergyBytes, err := os.ReadFile(filepath.Join(domainPath, "max_energy_range_uj"))
			var maxEnergy uint64
			if err == nil {
				maxEnergy, _ = strconv.ParseUint(strings.TrimSpace(string(maxEnergyBytes)), 10, 64)
			}
			
			domainPaths[entry.Name()] = domainPath
			maxEnergies[entry.Name()] = maxEnergy
		}
	}

	if len(domainPaths) == 0 {
		return nil, fmt.Errorf("no RAPL domains found in %s", basePath)
	}

	return &raplReader{
		domainPaths: domainPaths,
		maxEnergies: maxEnergies,
	}, nil
}

func (r *raplReader) Read() (Reading, error) {
	now := time.Now()
	domains := make(map[string]DomainData)

	for id, path := range r.domainPaths {
		valBytes, err := os.ReadFile(filepath.Join(path, "energy_uj"))
		if err != nil {
			return Reading{}, fmt.Errorf("failed to read energy for domain %s: %w", id, err)
		}
		
		val, err := strconv.ParseUint(strings.TrimSpace(string(valBytes)), 10, 64)
		if err != nil {
			return Reading{}, err
		}

		domains[id] = DomainData{
			ID:               id,
			EnergyMicroJoules: val,
			MaxEnergyRangeUJ:  r.maxEnergies[id],
		}
	}

	return Reading{
		Timestamp: now,
		Domains:   domains,
	}, nil
}

// CalculateDelta returns the total energy delta in microjoules across all domains
func CalculateDelta(before, after Reading) uint64 {
	var totalDelta uint64

	for id, afterDomain := range after.Domains {
		if beforeDomain, ok := before.Domains[id]; ok {
			if afterDomain.EnergyMicroJoules >= beforeDomain.EnergyMicroJoules {
				totalDelta += afterDomain.EnergyMicroJoules - beforeDomain.EnergyMicroJoules
			} else {
				// Wrap-around
				if afterDomain.MaxEnergyRangeUJ > 0 {
					totalDelta += (afterDomain.MaxEnergyRangeUJ - beforeDomain.EnergyMicroJoules) + afterDomain.EnergyMicroJoules
				} else {
					// If max range is 0/unknown, we can't reliably calculate, just skip or add 0
				}
			}
		}
	}

	return totalDelta
}
