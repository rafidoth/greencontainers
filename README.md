# Green Containers

`greencontainers` is a production-quality, automated Docker lifecycle energy profiling framework designed for academic research on software energy consumption.

## Research Purpose
The objective of this tool is to provide a controlled environment to measure the hardware-level energy consumption of Docker containers across various lifecycle stages (Pull, Build, Start, Idle, Stop, Cleanup). This enables comparative analysis of Docker base images (e.g., Ubuntu, Alpine, Distroless, Scratch) to understand their energy footprints.

## Architecture
The framework is built in Go and follows this pipeline:
1. **Experiment Config**: Parses CLI flags and setup parameters.
2. **Docker Controller**: Uses the official Docker Go SDK to orchestrate container lifecycles.
3. **RAPL Reader**: Reads hardware energy counters directly from `/sys/class/powercap/intel-rapl/`.
4. **Baseline Corrector**: Measures background host idle power to isolate stage-specific energy.
5. **Lifecycle Profiler**: Executes parameterized trials, collects measurements, and calculates deltas.
6. **Output Generator**: Writes results to CSV and JSON formats for subsequent statistical analysis (e.g., using Python/pandas).

## Requirements
- Target OS: Linux
- Go 1.21+
- Docker Engine installed and running
- Intel CPU with RAPL support

## Installation

```bash
git clone <repository>
cd greencontainers
go build -o greencontainers ./cmd/greencontainers
```

## Permissions

### RAPL Permissions
To read RAPL energy counters without running as root, adjust the permissions of the powercap sysfs directories:

```bash
sudo chmod -R a+r /sys/class/powercap/intel-rapl/
```

### Docker Permissions
Ensure your user is part of the `docker` group to interact with the Docker daemon without `sudo`:

```bash
sudo usermod -aG docker $USER
```

## CLI Usage

Verify environment:
```bash
./greencontainers check
```

Print current RAPL readings:
```bash
./greencontainers rapl
```

Perform baseline measurement:
```bash
./greencontainers baseline --duration 60s
```

Run a full experiment:
```bash
./greencontainers run \
  --images ubuntu:latest,alpine:latest \
  --trials 30 \
  --idle 60s \
  --baseline 60s \
  --randomize \
  --cold-pull=true \
  --output ./results
```

## Experiment Methodology
- **Trials**: Each image goes through multiple trials to gather statistically significant samples.
- **Randomization**: Trial execution order can be randomized to reduce the impact of time-dependent external variables (e.g., thermal throttling, background OS tasks).
- **Cold Pull**: Optionally removes images before the "Pull" stage to measure actual network/disk energy rather than cached operations.
- **Data Structuring**: Raw measurements and corrected metrics are preserved. The Go framework outputs clean observational data; statistical derivation (ANOVA, regression) should be done post-hoc in tools like R or Pandas.

## Lifecycle Definitions
1. **Pull**: Downloads the image from the registry. If `--cold-pull` is set, the image is deleted first.
2. **Build**: Constructs the image (if a Dockerfile is provided). Measures Docker assembly energy, *not* application compilation energy (unless compilation happens in the Dockerfile).
3. **Start**: Instantiates the container running a deterministic idle command (`sleep infinity`).
4. **Idle**: Measures energy consumption of the running container over a configured duration with no active workload.
5. **Stop**: Gracefully terminates the container.
6. **Cleanup**: Removes the container and optionally the image.

## Baseline Correction
Background host activity inevitably affects raw RAPL readings. The framework supports baseline correction:
1. It measures idle host power ($P_{baseline}$) before the experiment.
2. For any stage with duration $T_{stage}$, background energy is $E_{background} = P_{baseline} \times T_{stage}$.
3. Corrected energy is $E_{corrected} = E_{raw} - E_{background}$ (clamped to 0).
Both raw and corrected values are recorded.

## CSV Schema
The generated CSV includes comprehensive metadata, timestamps, raw energy ($\mu J$ and $J$), baseline parameters, corrected energy, average power ($W$), and CPU/Memory monitoring samples (average/peak).

## JSON Schema
Structured output containing experiment configuration, system metadata (Docker version, etc.), and nested trial/stage data.

## Important Limitations
- **Hardware-Domain Measurement**: RAPL measures total CPU package/core/DRAM energy. It **cannot** provide exact per-container energy attribution.
- **Background Interference**: System daemons and network operations impact readings. Baseline correction mitigates but does not eliminate this.
- **Network Variance**: "Pull" stage measurements are highly sensitive to network latency and bandwidth at the time of execution.
- **Interpretation**: Results must be interpreted as controlled *host-level* energy observations induced by Docker operations.
