package output

import (
	"encoding/csv"
	"fmt"
	"os"

	"greencontainers/internal/profiler"
)

func WriteCSV(path string, res profiler.ExperimentResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	headers := []string{
		"experiment_id",
		"trial_id",
		"trial_number",
		"base_image",
		"image",
		"image_tag",
		"image_digest",
		"image_id",
		"image_size_bytes",
		"image_size_mb",
		"image_layers",
		"stage",
		"stage_name",
		"stage_type",
		"started_at",
		"finished_at",
		"duration_ms",
		"rapl_before_uj",
		"rapl_after_uj",
		"energy_raw_uj",
		"energy_raw_j",
		"baseline_power_w",
		"background_energy_j",
		"energy_corrected_j",
		"average_power_w",
		"cpu_avg_percent",
		"cpu_peak_percent",
		"memory_avg_mb",
		"memory_peak_mb",
		"k6_rate_target",
		"k6_throughput_actual",
		"k6_avg_latency_ms",
		"k6_p95_latency_ms",
		"k6_p99_latency_ms",
		"docker_version",
		"success",
		"error",
	}

	if err := w.Write(headers); err != nil {
		return err
	}

	for _, s := range res.Stages {
		// Calculate total RAPL before/after sums across all domains
		var beforeTotal, afterTotal uint64
		for _, d := range s.RAPLBefore.Domains {
			beforeTotal += d.EnergyMicroJoules
		}
		for _, d := range s.RAPLAfter.Domains {
			afterTotal += d.EnergyMicroJoules
		}

		// k6 fields (empty if not a k6 stage)
		k6RateTarget := ""
		k6Throughput := ""
		k6AvgLatency := ""
		k6P95Latency := ""
		k6P99Latency := ""

		if s.K6RateTarget > 0 {
			k6RateTarget = fmt.Sprintf("%d", s.K6RateTarget)
		}
		if s.K6Result != nil {
			k6Throughput = fmt.Sprintf("%.2f", s.K6Result.Throughput)
			k6AvgLatency = fmt.Sprintf("%.3f", s.K6Result.AvgLatency)
			k6P95Latency = fmt.Sprintf("%.3f", s.K6Result.P95Latency)
			k6P99Latency = fmt.Sprintf("%.3f", s.K6Result.P99Latency)
		}

		row := []string{
			s.ExperimentID,
			s.TrialID,
			fmt.Sprintf("%d", s.TrialNumber),
			s.BaseImage,
			s.ImageMetadata.Name,
			s.ImageMetadata.Tag,
			s.ImageMetadata.Digest,
			s.ImageMetadata.ID,
			fmt.Sprintf("%d", s.ImageMetadata.SizeBytes),
			fmt.Sprintf("%.2f", s.ImageMetadata.SizeMB),
			fmt.Sprintf("%d", s.ImageMetadata.NumLayers),
			string(s.Stage),
			s.StageName,
			s.StageType,
			s.StartedAt.Format("2006-01-02T15:04:05.999Z07:00"),
			s.FinishedAt.Format("2006-01-02T15:04:05.999Z07:00"),
			fmt.Sprintf("%d", s.Duration.Milliseconds()),
			fmt.Sprintf("%d", beforeTotal),
			fmt.Sprintf("%d", afterTotal),
			fmt.Sprintf("%d", s.RawEnergyMicroJoules),
			fmt.Sprintf("%.6f", s.RawEnergyJoules),
			fmt.Sprintf("%.6f", s.BaselinePowerWatts),
			fmt.Sprintf("%.6f", s.BackgroundEnergyJ),
			fmt.Sprintf("%.6f", s.EnergyCorrectedJ),
			fmt.Sprintf("%.6f", s.AveragePowerWatts),
			fmt.Sprintf("%.2f", s.CPUAvgPercent),
			fmt.Sprintf("%.2f", s.CPUPeakPercent),
			fmt.Sprintf("%.2f", s.MemAvgMB),
			fmt.Sprintf("%.2f", s.MemPeakMB),
			k6RateTarget,
			k6Throughput,
			k6AvgLatency,
			k6P95Latency,
			k6P99Latency,
			res.DockerVer,
			fmt.Sprintf("%t", s.Success),
			s.Error,
		}

		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
