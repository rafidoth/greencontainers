package output

import (
	"encoding/json"
	"os"

	"greencontainers/internal/profiler"
)

func WriteJSON(path string, res profiler.ExperimentResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
