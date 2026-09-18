package profiler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"greencontainers/internal/baseline"
	"greencontainers/internal/config"
	"greencontainers/internal/docker"
	"greencontainers/internal/loadgen"
	"greencontainers/internal/metrics"
	"greencontainers/internal/rapl"
	"greencontainers/internal/readiness"
)

type Stage string

const (
	StagePull     Stage = "Pull"
	StageBuild    Stage = "Build"
	StageStart    Stage = "Start"
	StageIdle     Stage = "Idle"
	StageWorkload Stage = "Workload"
	StageStop     Stage = "Stop"
	StageCleanup  Stage = "Cleanup"
)

type StageResult struct {
	ExperimentID         string
	TrialID              string
	TrialNumber          int
	ImageMetadata        docker.ImageMetadata

	// Stage identification
	Stage                Stage
	StageName            string // Human-readable name from YAML config (e.g. "heavy-load")
	StageType            string // "idle", "wrk2", "pull", "build", "start", "stop", "cleanup"

	StartedAt            time.Time
	FinishedAt           time.Time
	Duration             time.Duration

	RAPLBefore           rapl.Reading
	RAPLAfter            rapl.Reading
	RawEnergyMicroJoules uint64
	RawEnergyJoules      float64

	BaselinePowerWatts   float64
	BackgroundEnergyJ    float64
	EnergyCorrectedJ     float64

	AveragePowerWatts    float64

	CPUAvgPercent        float64
	CPUPeakPercent       float64
	MemAvgMB             float64
	MemPeakMB            float64

	// k6-specific results (nil for non-k6 stages)
	K6Result             *loadgen.K6Result
	K6RateTarget         int

	// Base image used for this experiment (from config)
	BaseImage            string

	Success              bool
	Error                string
}

type ExperimentResult struct {
	ExperimentID string
	Config       config.Config
	DockerVer    string
	Baseline     baseline.Result
	Stages       []StageResult
}

type Profiler struct {
	cfg        config.Config
	dockerCtrl docker.Controller
	raplReader rapl.Reader
}

func NewProfiler(cfg config.Config, dockerCtrl docker.Controller, raplReader rapl.Reader) *Profiler {
	return &Profiler{
		cfg:        cfg,
		dockerCtrl: dockerCtrl,
		raplReader: raplReader,
	}
}

// MeasureStage wraps an operation in RAPL energy measurement and CPU/memory monitoring.
func (p *Profiler) MeasureStage(ctx context.Context, stage Stage, op func() error) StageResult {
	slog.Info("stage started", "stage", stage)

	before, err := p.raplReader.Read()
	if err != nil {
		return StageResult{Stage: stage, Success: false, Error: fmt.Sprintf("failed to read RAPL before: %v", err)}
	}

	monitor := metrics.StartMonitor(p.cfg.CPUSamplingInterval, p.cfg.MemorySamplingInterval)

	errOp := op()

	summary := monitor.Stop()

	after, err := p.raplReader.Read()
	if err != nil {
		errStr := fmt.Sprintf("failed to read RAPL after: %v", err)
		if errOp != nil {
			errStr = fmt.Sprintf("%s; op err: %v", errStr, errOp)
		}
		return StageResult{Stage: stage, Success: false, Error: errStr}
	}

	deltaUJ := rapl.CalculateDelta(before, after)
	deltaJ := float64(deltaUJ) / 1000000.0
	duration := after.Timestamp.Sub(before.Timestamp)
	avgPower := 0.0
	if duration.Seconds() > 0 {
		avgPower = deltaJ / duration.Seconds()
	}

	errStr := ""
	if errOp != nil {
		errStr = errOp.Error()
	}

	slog.Info("stage completed",
		"stage", stage,
		"energy_j", deltaJ,
		"duration", duration,
		"success", errOp == nil,
	)

	return StageResult{
		Stage:                stage,
		StartedAt:            before.Timestamp,
		FinishedAt:           after.Timestamp,
		Duration:             duration,
		RAPLBefore:           before,
		RAPLAfter:            after,
		RawEnergyMicroJoules: deltaUJ,
		RawEnergyJoules:      deltaJ,
		AveragePowerWatts:    avgPower,
		CPUAvgPercent:        summary.CPUAvgPercent,
		CPUPeakPercent:       summary.CPUPeakPercent,
		MemAvgMB:             summary.MemAvgMB,
		MemPeakMB:            summary.MemPeakMB,
		Success:              errOp == nil,
		Error:                errStr,
	}
}

// RunExperimentYAML runs the new YAML-based experiment lifecycle:
//
//	Baseline → Build → For each trial: Start → Readiness → Stages → Stop → Cleanup
func (p *Profiler) RunExperimentYAML(ctx context.Context, baselineRes baseline.Result) (ExperimentResult, error) {
	dockerVer, err := p.dockerCtrl.Version(ctx)
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("failed to get docker version: %w", err)
	}

	res := ExperimentResult{
		ExperimentID: p.cfg.ExperimentID,
		Config:       p.cfg,
		DockerVer:    dockerVer,
		Baseline:     baselineRes,
		Stages:       make([]StageResult, 0),
	}

	// --- Pull Stage ---
	if p.cfg.Build.ColdPull && p.cfg.Build.BaseImage != "" {
		slog.Info("performing cold pull of base image", "base_image", p.cfg.Build.BaseImage)
		
		// Ensure it's removed first (unmeasured setup)
		_ = p.dockerCtrl.RemoveImage(ctx, p.cfg.Build.BaseImage)

		pullRes := p.MeasureStage(ctx, StagePull, func() error {
			return p.dockerCtrl.Pull(ctx, p.cfg.Build.BaseImage)
		})
		pullRes.StageName = "pull"
		pullRes.StageType = "pull"
		pullRes.BaseImage = p.cfg.Build.BaseImage
		p.enrichStage(&pullRes, p.cfg.ExperimentID, p.cfg.ExperimentID+"-pull", 0, docker.ImageMetadata{}, baselineRes.PowerWatts)
		res.Stages = append(res.Stages, pullRes)

		if !pullRes.Success {
			return res, fmt.Errorf("pull failed: %s", pullRes.Error)
		}
	}

	// --- Build Stage ---
	slog.Info("building image",
		"context", p.cfg.Build.Context,
		"dockerfile", p.cfg.Build.Dockerfile,
		"base_image", p.cfg.Build.BaseImage,
		"tag", p.cfg.Build.Tag,
	)

	buildArgs := map[string]*string{
		"BASE_IMAGE": &p.cfg.Build.BaseImage,
	}

	buildRes := p.MeasureStage(ctx, StageBuild, func() error {
		return p.dockerCtrl.Build(ctx, p.cfg.Build.Dockerfile, p.cfg.Build.Context, p.cfg.Build.Tag, true, buildArgs)
	})
	buildRes.StageName = "build"
	buildRes.StageType = "build"
	buildRes.BaseImage = p.cfg.Build.BaseImage
	p.enrichStage(&buildRes, p.cfg.ExperimentID, p.cfg.ExperimentID+"-build", 0, docker.ImageMetadata{}, baselineRes.PowerWatts)
	res.Stages = append(res.Stages, buildRes)

	if !buildRes.Success {
		return res, fmt.Errorf("build failed: %s", buildRes.Error)
	}

	// Inspect the built image
	imgMeta, err := p.dockerCtrl.InspectImage(ctx, p.cfg.Build.Tag)
	if err != nil {
		slog.Warn("failed to inspect built image", "tag", p.cfg.Build.Tag, "error", err)
	}

	// --- Trial Loop ---
	for trial := 1; trial <= p.cfg.Trials; trial++ {
		trialID := fmt.Sprintf("%s-t%d", p.cfg.ExperimentID, trial)
		containerName := fmt.Sprintf("greencontainers-%s", trialID)

		slog.Info("starting trial", "trial_id", trialID, "trial_num", trial, "total_trials", p.cfg.Trials)

		// Use a closure to guarantee cleanup via defer
		trialErr := func() error {
			// --- Start Stage ---
			hostPort := fmt.Sprintf("%d", p.cfg.Container.HostPort)
			containerPort := fmt.Sprintf("%d", p.cfg.Container.ContainerPort)

			startRes := p.MeasureStage(ctx, StageStart, func() error {
				return p.dockerCtrl.Start(ctx, containerName, p.cfg.Build.Tag, nil, hostPort, containerPort)
			})
			startRes.StageName = "start"
			startRes.StageType = "start"
			startRes.BaseImage = p.cfg.Build.BaseImage
			p.enrichStage(&startRes, p.cfg.ExperimentID, trialID, trial, imgMeta, baselineRes.PowerWatts)
			res.Stages = append(res.Stages, startRes)

			// Guarantee cleanup even if stages fail
			defer func() {
				slog.Info("cleaning up trial", "trial_id", trialID)

				stopRes := p.MeasureStage(ctx, StageStop, func() error {
					return p.dockerCtrl.Stop(ctx, containerName)
				})
				stopRes.StageName = "stop"
				stopRes.StageType = "stop"
				stopRes.BaseImage = p.cfg.Build.BaseImage
				p.enrichStage(&stopRes, p.cfg.ExperimentID, trialID, trial, imgMeta, baselineRes.PowerWatts)
				res.Stages = append(res.Stages, stopRes)

				cleanRes := p.MeasureStage(ctx, StageCleanup, func() error {
					return p.dockerCtrl.RemoveContainer(ctx, containerName)
				})
				cleanRes.StageName = "cleanup"
				cleanRes.StageType = "cleanup"
				cleanRes.BaseImage = p.cfg.Build.BaseImage
				p.enrichStage(&cleanRes, p.cfg.ExperimentID, trialID, trial, imgMeta, baselineRes.PowerWatts)
				res.Stages = append(res.Stages, cleanRes)
			}()

			if !startRes.Success {
				return fmt.Errorf("start failed: %s", startRes.Error)
			}

			// --- Readiness Check (not measured) ---
			if p.cfg.Endpoints.Health != "" {
				slog.Info("waiting for readiness", "health_url", p.cfg.Endpoints.Health)
				if err := readiness.WaitForReady(ctx, p.cfg.Endpoints.Health, 30*time.Second, 500*time.Millisecond); err != nil {
					return fmt.Errorf("readiness check failed: %w", err)
				}
			}

			// --- Execute each configured stage ---
			for i, stageCfg := range p.cfg.Stages {
				slog.Info("executing stage",
					"stage_name", stageCfg.Name,
					"stage_type", stageCfg.Type,
					"stage_index", i+1,
					"total_stages", len(p.cfg.Stages),
				)

				switch stageCfg.Type {
				case "idle":
					idleRes := p.MeasureStage(ctx, StageIdle, func() error {
						select {
						case <-time.After(stageCfg.ParsedDuration):
							return nil
						case <-ctx.Done():
							return ctx.Err()
						}
					})
					idleRes.StageName = stageCfg.Name
					idleRes.StageType = "idle"
					idleRes.BaseImage = p.cfg.Build.BaseImage
					p.enrichStage(&idleRes, p.cfg.ExperimentID, trialID, trial, imgMeta, baselineRes.PowerWatts)
					res.Stages = append(res.Stages, idleRes)

				case "k6":
					var k6Res *loadgen.K6Result
					workloadRes := p.MeasureStage(ctx, StageWorkload, func() error {
						k6cfg := loadgen.K6Config{
							VUs:       stageCfg.Connections,
							Duration:  stageCfg.ParsedDuration,
							Rate:      stageCfg.Rate,
							TargetURL: p.cfg.Endpoints.Target,
						}
						result, err := loadgen.Run(ctx, k6cfg)
						if err != nil {
							return err
						}
						k6Res = &result
						return nil
					})
					workloadRes.StageName = stageCfg.Name
					workloadRes.StageType = "k6"
					workloadRes.K6Result = k6Res
					workloadRes.K6RateTarget = stageCfg.Rate
					workloadRes.BaseImage = p.cfg.Build.BaseImage
					p.enrichStage(&workloadRes, p.cfg.ExperimentID, trialID, trial, imgMeta, baselineRes.PowerWatts)
					res.Stages = append(res.Stages, workloadRes)

				default:
					slog.Warn("unknown stage type, skipping", "type", stageCfg.Type)
				}

				// --- Cooldown between stages (not measured) ---
				if i < len(p.cfg.Stages)-1 && p.cfg.CooldownDuration > 0 {
					slog.Info("cooldown", "duration", p.cfg.CooldownDuration)
					select {
					case <-time.After(p.cfg.CooldownDuration):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}

			return nil
		}()

		if trialErr != nil {
			slog.Error("trial failed", "trial_id", trialID, "err", trialErr)
			// Continue to next trial rather than aborting the entire experiment
		}

		slog.Info("completed trial", "trial_id", trialID)
	}

	return res, nil
}

// RunExperiment is the legacy experiment runner for the old CLI-based flow.
// Kept for backward compatibility when no YAML config is provided.
func (p *Profiler) RunExperiment(ctx context.Context, expID string, baselineRes baseline.Result) (ExperimentResult, error) {
	dockerVer, err := p.dockerCtrl.Version(ctx)
	if err != nil {
		return ExperimentResult{}, fmt.Errorf("failed to get docker version: %w", err)
	}

	res := ExperimentResult{
		ExperimentID: expID,
		Config:       p.cfg,
		DockerVer:    dockerVer,
		Baseline:     baselineRes,
		Stages:       make([]StageResult, 0),
	}

	for trial := 1; trial <= p.cfg.Trials; trial++ {
		for _, img := range p.cfg.Images {
			trialID := fmt.Sprintf("%s-t%d-%s", expID, trial, img)
			containerName := fmt.Sprintf("greencontainers-%s", trialID)

			slog.Info("starting legacy trial", "trial_id", trialID, "image", img)

			// Pull
			pullRes := p.MeasureStage(ctx, StagePull, func() error {
				if p.cfg.ColdPull {
					_ = p.dockerCtrl.RemoveImage(ctx, img)
				}
				return p.dockerCtrl.Pull(ctx, img)
			})
			imgMeta, _ := p.dockerCtrl.InspectImage(ctx, img)
			pullRes.StageName = "pull"
			pullRes.StageType = "pull"
			p.enrichStage(&pullRes, expID, trialID, trial, imgMeta, baselineRes.PowerWatts)
			res.Stages = append(res.Stages, pullRes)

			// Start
			startRes := p.MeasureStage(ctx, StageStart, func() error {
				cmd := []string{"sleep", "infinity"}
				return p.dockerCtrl.Start(ctx, containerName, img, cmd, "", "")
			})
			startRes.StageName = "start"
			startRes.StageType = "start"
			p.enrichStage(&startRes, expID, trialID, trial, imgMeta, baselineRes.PowerWatts)
			res.Stages = append(res.Stages, startRes)

			// Idle
			if startRes.Success {
				idleRes := p.MeasureStage(ctx, StageIdle, func() error {
					select {
					case <-time.After(p.cfg.IdleDuration):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				idleRes.StageName = "idle"
				idleRes.StageType = "idle"
				p.enrichStage(&idleRes, expID, trialID, trial, imgMeta, baselineRes.PowerWatts)
				res.Stages = append(res.Stages, idleRes)
			}

			// Stop
			stopRes := p.MeasureStage(ctx, StageStop, func() error {
				return p.dockerCtrl.Stop(ctx, containerName)
			})
			stopRes.StageName = "stop"
			stopRes.StageType = "stop"
			p.enrichStage(&stopRes, expID, trialID, trial, imgMeta, baselineRes.PowerWatts)
			res.Stages = append(res.Stages, stopRes)

			// Cleanup
			cleanRes := p.MeasureStage(ctx, StageCleanup, func() error {
				err := p.dockerCtrl.RemoveContainer(ctx, containerName)
				if p.cfg.ColdPull {
					_ = p.dockerCtrl.RemoveImage(ctx, img)
				}
				return err
			})
			cleanRes.StageName = "cleanup"
			cleanRes.StageType = "cleanup"
			p.enrichStage(&cleanRes, expID, trialID, trial, imgMeta, baselineRes.PowerWatts)
			res.Stages = append(res.Stages, cleanRes)

			slog.Info("completed legacy trial", "trial_id", trialID)
		}
	}

	return res, nil
}

func (p *Profiler) enrichStage(res *StageResult, expID, trialID string, trialNum int, meta docker.ImageMetadata, baselinePower float64) {
	res.ExperimentID = expID
	res.TrialID = trialID
	res.TrialNumber = trialNum
	res.ImageMetadata = meta

	if p.cfg.EnableBaselineCorrection {
		res.BaselinePowerWatts = baselinePower
		res.BackgroundEnergyJ = baselinePower * res.Duration.Seconds()
		res.EnergyCorrectedJ = res.RawEnergyJoules - res.BackgroundEnergyJ
		if res.EnergyCorrectedJ < 0 {
			res.EnergyCorrectedJ = 0
		}
	}
}
