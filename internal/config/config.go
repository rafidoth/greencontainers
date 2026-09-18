package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the full experiment configuration parsed from a YAML file.
type Config struct {
	ExperimentID string       `yaml:"experiment_id"`
	Trials       int          `yaml:"trials"`
	Build        BuildConfig  `yaml:"build"`
	Container    ContainerCfg `yaml:"container"`
	Endpoints    Endpoints    `yaml:"endpoints"`
	Baseline     BaselineCfg  `yaml:"baseline"`
	Cooldown     string       `yaml:"cooldown"`
	Stages       []StageCfg   `yaml:"stages"`

	// Parsed durations (populated after loading)
	CooldownDuration         time.Duration `yaml:"-"`
	BaselineDuration         time.Duration `yaml:"-"`
	EnableBaselineCorrection bool          `yaml:"-"`

	// System monitoring intervals (hardcoded defaults)
	CPUSamplingInterval    time.Duration `yaml:"-"`
	MemorySamplingInterval time.Duration `yaml:"-"`

	// Legacy fields kept for backward compatibility with the old CLI flow
	Images           []string      `yaml:"-"`
	IdleDuration     time.Duration `yaml:"-"`
	OutputDirectory  string        `yaml:"output_directory"`
	ColdPull         bool          `yaml:"-"`
	Randomize        bool          `yaml:"-"`
	Dockerfile       string        `yaml:"-"`
	BuildContext     string        `yaml:"-"`
	ContainerCommand string        `yaml:"-"`
	ContainerPort    string        `yaml:"-"`
}

type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
	BaseImage  string `yaml:"base_image"`
	Tag        string `yaml:"tag"`
	ColdPull   bool   `yaml:"cold_pull"`
}

type ContainerCfg struct {
	HostPort      int `yaml:"host_port"`
	ContainerPort int `yaml:"container_port"`
}

type Endpoints struct {
	Health string `yaml:"health"`
	Target string `yaml:"target"`
}

type BaselineCfg struct {
	Duration string `yaml:"duration"`
}

type StageCfg struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // "idle" or "wrk2"
	Duration    string `yaml:"duration"`
	Rate        int    `yaml:"rate,omitempty"`
	Threads     int    `yaml:"threads,omitempty"`
	Connections int    `yaml:"connections,omitempty"`

	// Parsed duration (populated after loading)
	ParsedDuration time.Duration `yaml:"-"`
}

// LoadFromFile reads and parses a YAML experiment config file.
func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Apply defaults
	if cfg.Trials <= 0 {
		cfg.Trials = 1
	}
	if cfg.OutputDirectory == "" {
		cfg.OutputDirectory = "./results"
	}

	// Parse durations
	if cfg.Baseline.Duration != "" {
		cfg.BaselineDuration, err = time.ParseDuration(cfg.Baseline.Duration)
		if err != nil {
			return Config{}, fmt.Errorf("invalid baseline duration %q: %w", cfg.Baseline.Duration, err)
		}
		cfg.EnableBaselineCorrection = true
	} else {
		cfg.BaselineDuration = 60 * time.Second
		cfg.EnableBaselineCorrection = true
	}

	if cfg.Cooldown != "" {
		cfg.CooldownDuration, err = time.ParseDuration(cfg.Cooldown)
		if err != nil {
			return Config{}, fmt.Errorf("invalid cooldown duration %q: %w", cfg.Cooldown, err)
		}
	} else {
		cfg.CooldownDuration = 10 * time.Second
	}

	for i := range cfg.Stages {
		if cfg.Stages[i].Duration != "" {
			cfg.Stages[i].ParsedDuration, err = time.ParseDuration(cfg.Stages[i].Duration)
			if err != nil {
				return Config{}, fmt.Errorf("invalid stage %q duration %q: %w", cfg.Stages[i].Name, cfg.Stages[i].Duration, err)
			}
		}
		// Default k6 threads and connections
		if cfg.Stages[i].Type == "k6" {
			if cfg.Stages[i].Threads <= 0 {
				cfg.Stages[i].Threads = 2
			}
			if cfg.Stages[i].Connections <= 0 {
				cfg.Stages[i].Connections = 10
			}
		}
	}

	// Hardcoded monitoring intervals
	cfg.CPUSamplingInterval = 1 * time.Second
	cfg.MemorySamplingInterval = 1 * time.Second

	return cfg, nil
}

// DefaultConfig returns a minimal default config for backward compatibility
// with the old CLI-based flow (no YAML file).
func DefaultConfig() Config {
	return Config{
		Trials:                   1,
		IdleDuration:             60 * time.Second,
		OutputDirectory:          "./results",
		BaselineDuration:         60 * time.Second,
		EnableBaselineCorrection: true,
		CPUSamplingInterval:      1 * time.Second,
		MemorySamplingInterval:   1 * time.Second,
		ColdPull:                 true,
		Randomize:                false,
		CooldownDuration:         10 * time.Second,
	}
}
