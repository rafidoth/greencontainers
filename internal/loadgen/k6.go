package loadgen

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// K6Config holds the parameters for a k6 load test run.
type K6Config struct {
	VUs      int
	Duration time.Duration
	Rate     int
	TargetURL string
}

// K6Result holds the parsed output from a k6 run.
type K6Result struct {
	AvgLatency float64 `json:"avg_latency_ms"`
	P95Latency float64 `json:"p95_latency_ms"`
	P99Latency float64 `json:"p99_latency_ms"`
	Throughput float64 `json:"throughput_rps"`
	TotalReqs  int64   `json:"total_requests"`
}

// CheckK6Installed verifies that k6 is available on the system PATH.
func CheckK6Installed() error {
	_, err := exec.LookPath("k6")
	if err != nil {
		return fmt.Errorf("k6 not found on PATH: %w (install with: pacman -S k6)", err)
	}
	return nil
}

// Run executes a k6 load test using constant-arrival-rate and returns parsed results.
func Run(ctx context.Context, cfg K6Config) (K6Result, error) {
	// Create a temporary directory for k6 script and summary output
	tmpDir, err := os.MkdirTemp("", "greencontainers-k6-*")
	if err != nil {
		return K6Result{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "script.js")
	summaryPath := filepath.Join(tmpDir, "summary.json")

	// Generate the k6 script
	script := generateK6Script(cfg)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return K6Result{}, fmt.Errorf("failed to write k6 script: %w", err)
	}

	slog.Info("running k6",
		"rate", cfg.Rate,
		"vus", cfg.VUs,
		"duration", cfg.Duration,
		"url", cfg.TargetURL,
	)

	// Run k6
	args := []string{
		"run",
		"--summary-export", summaryPath,
		"--quiet",
		scriptPath,
	}

	cmd := exec.CommandContext(ctx, "k6", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return K6Result{}, fmt.Errorf("k6 failed: %w", err)
	}

	// Parse summary JSON
	result, err := parseSummary(summaryPath)
	if err != nil {
		return K6Result{}, fmt.Errorf("failed to parse k6 summary: %w", err)
	}

	slog.Info("k6 completed",
		"throughput", result.Throughput,
		"avg_latency_ms", result.AvgLatency,
		"p99_latency_ms", result.P99Latency,
		"total_reqs", result.TotalReqs,
	)

	return result, nil
}

// generateK6Script creates a k6 JavaScript test script using
// the constant-arrival-rate executor for precise request rate control.
func generateK6Script(cfg K6Config) string {
	durSeconds := int(cfg.Duration.Seconds())
	maxVUs := cfg.VUs
	if maxVUs < cfg.Rate {
		maxVUs = cfg.Rate // ensure enough VUs to sustain the target rate
	}

	return fmt.Sprintf(`
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  discardResponseBodies: true,
  scenarios: {
    constant_rate: {
      executor: 'constant-arrival-rate',
      rate: %d,
      timeUnit: '1s',
      duration: '%ds',
      preAllocatedVUs: %d,
      maxVUs: %d,
    },
  },
};

export default function () {
  const res = http.get('%s');
  check(res, {
    'status is 200': (r) => r.status === 200,
  });
}
`, cfg.Rate, durSeconds, cfg.VUs, maxVUs, cfg.TargetURL)
}

// k6SummaryJSON matches the structure of k6's --summary-export output.
type k6SummaryJSON struct {
	Metrics struct {
		HTTPReqDuration struct {
			Avg    float64            `json:"avg"`
			Min    float64            `json:"min"`
			Max    float64            `json:"max"`
			Med    float64            `json:"med"`
			Values map[string]float64 `json:"p(95)"`
		} `json:"http_req_duration"`
		HTTPReqs struct {
			Count int64   `json:"count"`
			Rate  float64 `json:"rate"`
		} `json:"http_reqs"`
	} `json:"metrics"`
}

// parseSummary reads and parses the k6 summary JSON export file.
func parseSummary(path string) (K6Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return K6Result{}, err
	}

	// k6 summary JSON has a flexible structure, parse it generically
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return K6Result{}, err
	}

	result := K6Result{}

	metrics, ok := raw["metrics"].(map[string]interface{})
	if !ok {
		return result, fmt.Errorf("missing 'metrics' in k6 summary")
	}

	// Parse http_req_duration
	if dur, ok := metrics["http_req_duration"].(map[string]interface{}); ok {
		if values, ok := dur["values"].(map[string]interface{}); ok {
			if avg, ok := values["avg"].(float64); ok {
				result.AvgLatency = avg
			}
			if p95, ok := values["p(95)"].(float64); ok {
				result.P95Latency = p95
			}
			if p99, ok := values["p(99)"].(float64); ok {
				result.P99Latency = p99
			}
		}
	}

	// Parse http_reqs
	if reqs, ok := metrics["http_reqs"].(map[string]interface{}); ok {
		if values, ok := reqs["values"].(map[string]interface{}); ok {
			if rate, ok := values["rate"].(float64); ok {
				result.Throughput = rate
			}
			if count, ok := values["count"].(float64); ok {
				result.TotalReqs = int64(count)
			}
		}
	}

	return result, nil
}
