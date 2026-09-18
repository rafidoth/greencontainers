package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"greencontainers/internal/baseline"
	"greencontainers/internal/config"
	"greencontainers/internal/docker"
	"greencontainers/internal/loadgen"
	"greencontainers/internal/output"
	"greencontainers/internal/profiler"
	"greencontainers/internal/rapl"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "check":
		runCheck()
	case "rapl":
		runRAPL()
	case "baseline":
		runBaseline(os.Args[2:])
	case "run":
		runExperiment(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: greencontainers <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  check     Verify Docker, RAPL, k6, and system permissions")
	fmt.Println("  rapl      Print current RAPL readings")
	fmt.Println("  baseline  Perform the baseline experiment")
	fmt.Println("  run       Perform the complete lifecycle experiment")
	fmt.Println("")
	fmt.Println("Run modes:")
	fmt.Println("  greencontainers run --config experiment.yaml   (YAML-based experiment)")
	fmt.Println("  greencontainers run --images ubuntu:latest     (legacy CLI-based experiment)")
}

func runCheck() {
	fmt.Println("Checking Docker...")
	ctx := context.Background()
	d, err := docker.NewController()
	if err != nil {
		fmt.Printf("  [FAIL] Failed to create Docker client: %v\n", err)
	} else {
		ver, err := d.Version(ctx)
		if err != nil {
			fmt.Printf("  [FAIL] Failed to connect to Docker daemon: %v\n", err)
		} else {
			fmt.Printf("  [OK] Docker is running (version %s)\n", ver)
		}
	}

	fmt.Println("Checking RAPL...")
	r, err := rapl.NewReader()
	if err != nil {
		fmt.Printf("  [FAIL] Failed to initialize RAPL: %v\n", err)
	} else {
		_, err := r.Read()
		if err != nil {
			fmt.Printf("  [FAIL] Failed to read RAPL: %v\n", err)
		} else {
			fmt.Printf("  [OK] RAPL is available and readable\n")
		}
	}

	fmt.Println("Checking k6...")
	if err := loadgen.CheckK6Installed(); err != nil {
		fmt.Printf("  [FAIL] %v\n", err)
	} else {
		fmt.Printf("  [OK] k6 is installed\n")
	}
}

func runRAPL() {
	r, err := rapl.NewReader()
	if err != nil {
		fmt.Printf("Error initializing RAPL: %v\n", err)
		os.Exit(1)
	}
	reading, err := r.Read()
	if err != nil {
		fmt.Printf("Error reading RAPL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Timestamp: %s\n", reading.Timestamp.Format(time.RFC3339))
	for id, domain := range reading.Domains {
		fmt.Printf("Domain %s:\n", id)
		fmt.Printf("  Energy (uJ): %d\n", domain.EnergyMicroJoules)
		fmt.Printf("  Energy (J):  %.6f\n", float64(domain.EnergyMicroJoules)/1000000.0)
		fmt.Printf("  Max Range:   %d\n", domain.MaxEnergyRangeUJ)
	}
}

func runBaseline(args []string) {
	fs := flag.NewFlagSet("baseline", flag.ExitOnError)
	durationStr := fs.String("duration", "60s", "Baseline measurement duration")
	fs.Parse(args)

	duration, err := time.ParseDuration(*durationStr)
	if err != nil {
		fmt.Printf("Invalid duration: %v\n", err)
		os.Exit(1)
	}

	r, err := rapl.NewReader()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	res, err := baseline.Measure(ctx, duration, r, time.Second, time.Second)
	if err != nil {
		fmt.Printf("Baseline measurement failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Baseline Results:\n")
	fmt.Printf("  Duration: %v\n", res.Duration)
	fmt.Printf("  Energy:   %.6f J\n", res.EnergyJoules)
	fmt.Printf("  Power:    %.6f W\n", res.PowerWatts)
	fmt.Printf("  CPU Avg:  %.2f%%\n", res.CPUAvgPercent)
	fmt.Printf("  Mem Avg:  %.2f MB\n", res.MemAvgMB)
}

func runExperiment(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)

	// New YAML-based config flag
	configFile := fs.String("config", "", "Path to YAML experiment config file")

	// Legacy CLI flags (used when --config is not provided)
	images := fs.String("images", "ubuntu:latest,alpine:latest", "Comma-separated list of images")
	trials := fs.Int("trials", 1, "Number of trials per image")
	idle := fs.String("idle", "60s", "Idle duration per trial")
	baselineDur := fs.String("baseline", "60s", "Baseline measurement duration")
	outDir := fs.String("output", "./results", "Output directory")
	randomize := fs.Bool("randomize", false, "Randomize trial order")
	coldPull := fs.Bool("cold-pull", true, "Force cold pull by removing image first")

	fs.Parse(args)

	ctx := context.Background()

	// Branch: YAML-based or legacy CLI-based
	if *configFile != "" {
		runYAMLExperiment(ctx, *configFile)
	} else {
		runLegacyExperiment(ctx, *images, *trials, *idle, *baselineDur, *outDir, *randomize, *coldPull)
	}
}

func runYAMLExperiment(ctx context.Context, configPath string) {
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		slog.Error("Failed to load config", "path", configPath, "err", err)
		os.Exit(1)
	}

	if cfg.OutputDirectory == "" {
		cfg.OutputDirectory = "./results"
	}

	fmt.Printf("Experiment: %s\n", cfg.ExperimentID)
	fmt.Printf("Base Image: %s\n", cfg.Build.BaseImage)
	fmt.Printf("Build Tag:  %s\n", cfg.Build.Tag)
	fmt.Printf("Trials:     %d\n", cfg.Trials)
	fmt.Printf("Stages:     %d\n", len(cfg.Stages))
	fmt.Printf("Cooldown:   %v\n", cfg.CooldownDuration)
	fmt.Printf("Baseline:   %v\n", cfg.BaselineDuration)
	fmt.Printf("Output:     %s\n", cfg.OutputDirectory)
	fmt.Println("----------------------------------------")

	d, err := docker.NewController()
	if err != nil {
		slog.Error("Failed to initialize Docker", "err", err)
		os.Exit(1)
	}

	r, err := rapl.NewReader()
	if err != nil {
		slog.Error("Failed to initialize RAPL", "err", err)
		os.Exit(1)
	}

	var bRes baseline.Result
	if cfg.EnableBaselineCorrection {
		slog.Info("Running baseline measurement", "duration", cfg.BaselineDuration)
		bRes, err = baseline.Measure(ctx, cfg.BaselineDuration, r, cfg.CPUSamplingInterval, cfg.MemorySamplingInterval)
		if err != nil {
			slog.Error("Baseline measurement failed", "err", err)
			os.Exit(1)
		}
		fmt.Printf("Baseline Power: %.6f W\n", bRes.PowerWatts)
		fmt.Println("----------------------------------------")
	}

	prof := profiler.NewProfiler(cfg, d, r)
	res, err := prof.RunExperimentYAML(ctx, bRes)
	if err != nil {
		slog.Error("Experiment failed", "err", err)
		os.Exit(1)
	}

	writeResults(cfg.OutputDirectory, cfg.ExperimentID, res)
}

func runLegacyExperiment(ctx context.Context, images string, trials int, idle, baselineDur, outDir string, randomize, coldPull bool) {
	idleDur, _ := time.ParseDuration(idle)
	baseDur, _ := time.ParseDuration(baselineDur)

	cfg := config.DefaultConfig()
	cfg.Images = strings.Split(images, ",")
	cfg.Trials = trials
	cfg.IdleDuration = idleDur
	cfg.BaselineDuration = baseDur
	cfg.OutputDirectory = outDir
	cfg.Randomize = randomize
	cfg.ColdPull = coldPull

	expID := fmt.Sprintf("exp-%d", time.Now().Unix())

	fmt.Printf("Experiment: %s\n", expID)
	fmt.Printf("Images: %s\n", images)
	fmt.Printf("Trials: %d\n", cfg.Trials)
	fmt.Printf("Idle: %v\n", cfg.IdleDuration)
	fmt.Printf("Baseline correction: %v\n", cfg.EnableBaselineCorrection)
	fmt.Printf("Output: %s\n", cfg.OutputDirectory)
	fmt.Println("----------------------------------------")

	d, err := docker.NewController()
	if err != nil {
		slog.Error("Failed to initialize Docker", "err", err)
		os.Exit(1)
	}

	r, err := rapl.NewReader()
	if err != nil {
		slog.Error("Failed to initialize RAPL", "err", err)
		os.Exit(1)
	}

	var bRes baseline.Result
	if cfg.EnableBaselineCorrection {
		slog.Info("Running baseline measurement", "duration", cfg.BaselineDuration)
		bRes, err = baseline.Measure(ctx, cfg.BaselineDuration, r, cfg.CPUSamplingInterval, cfg.MemorySamplingInterval)
		if err != nil {
			slog.Error("Baseline measurement failed", "err", err)
			os.Exit(1)
		}
	}

	prof := profiler.NewProfiler(cfg, d, r)
	res, err := prof.RunExperiment(ctx, expID, bRes)
	if err != nil {
		slog.Error("Experiment failed", "err", err)
		os.Exit(1)
	}

	writeResults(cfg.OutputDirectory, expID, res)
}

func writeResults(outDir, expID string, res profiler.ExperimentResult) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		slog.Error("Failed to create output directory", "err", err)
		os.Exit(1)
	}

	csvPath := filepath.Join(outDir, fmt.Sprintf("%s.csv", expID))
	if err := output.WriteCSV(csvPath, res); err != nil {
		slog.Error("Failed to write CSV", "err", err)
	} else {
		slog.Info("Wrote CSV", "path", csvPath)
	}

	jsonPath := filepath.Join(outDir, fmt.Sprintf("%s.json", expID))
	if err := output.WriteJSON(jsonPath, res); err != nil {
		slog.Error("Failed to write JSON", "err", err)
	} else {
		slog.Info("Wrote JSON", "path", jsonPath)
	}
}
