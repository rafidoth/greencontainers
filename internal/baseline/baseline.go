package baseline

import (
	"context"
	"log/slog"
	"time"

	"greencontainers/internal/metrics"
	"greencontainers/internal/rapl"
)

type Result struct {
	Duration      time.Duration
	EnergyJoules  float64
	PowerWatts    float64
	CPUAvgPercent float64
	MemAvgMB      float64
}

func Measure(ctx context.Context, duration time.Duration, raplReader rapl.Reader, cpuInterval, memInterval time.Duration) (Result, error) {
	slog.Info("starting baseline measurement", "duration", duration)

	before, err := raplReader.Read()
	if err != nil {
		return Result{}, err
	}

	monitor := metrics.StartMonitor(cpuInterval, memInterval)
	
	// Wait for the duration or context cancellation
	select {
	case <-time.After(duration):
	case <-ctx.Done():
		monitor.Stop()
		return Result{}, ctx.Err()
	}

	summary := monitor.Stop()
	
	after, err := raplReader.Read()
	if err != nil {
		return Result{}, err
	}

	deltaUJ := rapl.CalculateDelta(before, after)
	deltaJ := float64(deltaUJ) / 1000000.0
	actualDuration := after.Timestamp.Sub(before.Timestamp)
	
	powerWatts := deltaJ / actualDuration.Seconds()

	slog.Info("baseline measurement completed", 
		"energy_j", deltaJ, 
		"power_w", powerWatts, 
		"cpu_avg", summary.CPUAvgPercent,
	)

	return Result{
		Duration:      actualDuration,
		EnergyJoules:  deltaJ,
		PowerWatts:    powerWatts,
		CPUAvgPercent: summary.CPUAvgPercent,
		MemAvgMB:      summary.MemAvgMB,
	}, nil
}
