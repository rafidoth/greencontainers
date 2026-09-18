package readiness

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// WaitForReady polls the given health URL until it returns HTTP 200
// or the timeout is exceeded. This is used to ensure a container is
// fully booted before starting load tests.
func WaitForReady(ctx context.Context, healthURL string, timeout, interval time.Duration) error {
	slog.Info("waiting for container readiness", "url", healthURL, "timeout", timeout)

	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	client := &http.Client{Timeout: 2 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("readiness timeout after %v: %s never returned 200", timeout, healthURL)
		case <-ticker.C:
			resp, err := client.Get(healthURL)
			if err != nil {
				slog.Debug("readiness check failed (retrying)", "err", err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				slog.Info("container is ready", "url", healthURL)
				return nil
			}
			slog.Debug("readiness check returned non-200 (retrying)", "status", resp.StatusCode)
		}
	}
}
