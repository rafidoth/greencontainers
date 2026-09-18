//go:build linux

package metrics

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Summary struct {
	CPUAvgPercent float64
	CPUPeakPercent float64
	MemAvgMB       float64
	MemPeakMB      float64
}

type Monitor struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	
	cpuSamples []float64
	memSamples []float64
}

func StartMonitor(cpuInterval, memInterval time.Duration) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{
		cancel: cancel,
		cpuSamples: make([]float64, 0),
		memSamples: make([]float64, 0),
	}

	m.wg.Add(1)
	go m.loop(ctx, cpuInterval, memInterval)

	return m
}

func (m *Monitor) Stop() Summary {
	m.cancel()
	m.wg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	var cpuSum, memSum float64
	var cpuPeak, memPeak float64

	for _, c := range m.cpuSamples {
		cpuSum += c
		if c > cpuPeak {
			cpuPeak = c
		}
	}
	cpuAvg := 0.0
	if len(m.cpuSamples) > 0 {
		cpuAvg = cpuSum / float64(len(m.cpuSamples))
	}

	for _, mem := range m.memSamples {
		memSum += mem
		if mem > memPeak {
			memPeak = mem
		}
	}
	memAvg := 0.0
	if len(m.memSamples) > 0 {
		memAvg = memSum / float64(len(m.memSamples))
	}

	return Summary{
		CPUAvgPercent:  cpuAvg,
		CPUPeakPercent: cpuPeak,
		MemAvgMB:       memAvg,
		MemPeakMB:      memPeak,
	}
}

func (m *Monitor) loop(ctx context.Context, cpuInterval, memInterval time.Duration) {
	defer m.wg.Done()

	tickerCPU := time.NewTicker(cpuInterval)
	defer tickerCPU.Stop()
	
	tickerMem := time.NewTicker(memInterval)
	defer tickerMem.Stop()

	prevIdle, prevTotal := readCPU()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerCPU.C:
			idle, total := readCPU()
			idleTicks := float64(idle - prevIdle)
			totalTicks := float64(total - prevTotal)
			
			var usage float64
			if totalTicks > 0 {
				usage = 100.0 * (totalTicks - idleTicks) / totalTicks
			}
			
			prevIdle = idle
			prevTotal = total

			m.mu.Lock()
			m.cpuSamples = append(m.cpuSamples, usage)
			m.mu.Unlock()

		case <-tickerMem.C:
			usedMB := readMemMB()
			m.mu.Lock()
			m.memSamples = append(m.memSamples, usedMB)
			m.mu.Unlock()
		}
	}
}

func readCPU() (idle uint64, total uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				user, _ := strconv.ParseUint(fields[1], 10, 64)
				nice, _ := strconv.ParseUint(fields[2], 10, 64)
				system, _ := strconv.ParseUint(fields[3], 10, 64)
				idleVal, _ := strconv.ParseUint(fields[4], 10, 64)
				iowait, _ := strconv.ParseUint(fields[5], 10, 64)
				irq, _ := strconv.ParseUint(fields[6], 10, 64)
				softirq, _ := strconv.ParseUint(fields[7], 10, 64)

				idle = idleVal + iowait
				total = user + nice + system + idle + irq + softirq
			}
			break
		}
	}
	return
}

func readMemMB() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0.0
	}
	
	lines := strings.Split(string(data), "\n")
	var memTotal, memFree, buffers, cached uint64

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		
		switch fields[0] {
		case "MemTotal:":
			memTotal = val
		case "MemFree:":
			memFree = val
		case "Buffers:":
			buffers = val
		case "Cached:":
			cached = val
		}
	}

	usedKB := memTotal - memFree - buffers - cached
	return float64(usedKB) / 1024.0
}
