package profiling

import (
	"encoding/csv"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/config"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/shirou/gopsutil/cpu"
	"github.com/sirupsen/logrus"
)

var globalMonitorParams *config.PerformanceMonitor

// SetMonitorParams initializes the global PerformanceMonitor from Config

// GetMonitorParams returns the PerformanceMonitor, with defaults if unset
func GetMonitorParams() *config.PerformanceMonitor {
	if globalMonitorParams == nil {
		return &config.PerformanceMonitor{
			Active:         false,
			SampleDuration: 1 * time.Second,
			ProfileDir:     "/tmp",
			Profile:        "prover-rounds",
		}
	}
	return globalMonitorParams
}

// PerformanceLog captures performance metrics
type PerformanceLog struct {
	Description string
	ProfilePath string

	StartTime time.Time
	StopTime  time.Time

	// CPU Usage Stats
	CpuUsageEachSeconds []float64  // CPU usage per second
	CpuUsageStats       [3]float64 // [min, avg, max] in percent

	// Memory usage per second in GiB
	MemoryAllocatedPerSecondGiB        []float64
	MemoryInUsePerSecondGiB            []float64
	MemoryGCNotDeallocatedPerSecondGiB []float64

	// Memory usage stats [min, avg, max] in GiB
	MemoryInUseStatsGiB            [3]float64
	MemoryAllocatedStatsGiB        [3]float64
	MemoryGCNotDeallocatedStatsGiB [3]float64

	// Profiling files
	cpuProfile *os.File
	memProfile *os.File
}

// performanceMonitor manages the collection of performance metrics
type performanceMonitor struct {
	log      *PerformanceLog
	stopChan chan struct{}
}

// StartPerformanceMonitor initializes and starts performance monitoring
// and samples at the specified sampleRate. If flame graph is not needed
// then empty string value may be passed
func StartPerformanceMonitor(description string, sampleRate time.Duration, profilePath string) (*performanceMonitor, error) {

	m := &performanceMonitor{
		log: &PerformanceLog{
			Description: description,
			ProfilePath: profilePath,
			StartTime:   time.Now(),
		},
		stopChan: make(chan struct{}),
	}

	if profilePath != "" {
		if err := os.MkdirAll(profilePath, 0755); err != nil {
			return nil, err
		}
		// Start CPU profiling
		cpuProfilePath := path.Join(m.log.ProfilePath, "cpu-profile.pb.gz")
		if f, err := os.Create(cpuProfilePath); err != nil {
			return nil, err
		} else {
			pprof.StartCPUProfile(f)
			m.log.cpuProfile = f
		}

		// Start memory profiling
		memProfilePath := path.Join(m.log.ProfilePath, "mem-profile.pb.gz")
		if f, err := os.Create(memProfilePath); err != nil {
			return nil, err
		} else {
			m.log.memProfile = f
		}
	}

	// Start sampling every `sampleRate`
	go m.sample(sampleRate)
	return m, nil
}

// Stop ends performance monitoring and returns the collected metrics
func (m *performanceMonitor) Stop() (*PerformanceLog, error) {
	close(m.stopChan)
	m.log.StopTime = time.Now()

	// Stop CPU profiling
	if m.log.cpuProfile != nil {
		pprof.StopCPUProfile()
		m.log.cpuProfile.Close()

	}

	// Write memory profile
	if m.log.memProfile != nil {
		pprof.WriteHeapProfile(m.log.memProfile)
		m.log.memProfile.Close()

	}

	m.calculateStats()
	return m.log, nil
}

// sample collects performance metrics at regular intervals
// sample collects CPU and memory performance metrics at regular intervals
func (m *performanceMonitor) sample(sampleRate time.Duration) {
	ticker := time.NewTicker(sampleRate)
	defer ticker.Stop()

	var maxUsage float64 = float64(runtime.NumCPU()) * 100
	var memStats runtime.MemStats
	for {
		select {
		case <-ticker.C:
			// Instant CPU usage per core
			percent, err := cpu.Percent(0, true)
			if err != nil {
				logrus.Fatal("error collecting CPU usage:", err)
			}

			cpuUsage := utils.SumFloat64(percent)
			if cpuUsage > maxUsage {
				logrus.Fatalf("error cpu usage:%.2f cannot exceed max. theoretical limit: %.2f", cpuUsage, maxUsage)
			}

			m.log.CpuUsageEachSeconds = append(m.log.CpuUsageEachSeconds, cpuUsage)

			// Get memory stats
			runtime.ReadMemStats(&memStats)
			memAllocatedGiB := utils.BytesToGiB(memStats.Alloc)
			memInUseGiB := utils.BytesToGiB(memStats.HeapInuse)
			memGCNotDeallocatedGiB := utils.BytesToGiB(memStats.HeapIdle - memStats.HeapReleased)

			m.log.MemoryAllocatedPerSecondGiB = append(m.log.MemoryAllocatedPerSecondGiB, memAllocatedGiB)
			m.log.MemoryInUsePerSecondGiB = append(m.log.MemoryInUsePerSecondGiB, memInUseGiB)
			m.log.MemoryGCNotDeallocatedPerSecondGiB = append(m.log.MemoryGCNotDeallocatedPerSecondGiB, memGCNotDeallocatedGiB)

		case <-m.stopChan:
			return
		}
	}
}

// calculateStats computes min, avg, and max for CPU and memory usage
func (m *performanceMonitor) calculateStats() {
	if len(m.log.CpuUsageEachSeconds) > 0 {
		min, avg, max := utils.CalculateMinAvgMax(m.log.CpuUsageEachSeconds)
		m.log.CpuUsageStats = [3]float64{min, avg, max}
	}

	if len(m.log.MemoryInUsePerSecondGiB) > 0 {
		min, avg, max := utils.CalculateMinAvgMax(m.log.MemoryInUsePerSecondGiB)
		m.log.MemoryInUseStatsGiB = [3]float64{min, avg, max}
	}

	if len(m.log.MemoryAllocatedPerSecondGiB) > 0 {
		min, avg, max := utils.CalculateMinAvgMax(m.log.MemoryAllocatedPerSecondGiB)
		m.log.MemoryAllocatedStatsGiB = [3]float64{min, avg, max}
	}

	if len(m.log.MemoryGCNotDeallocatedPerSecondGiB) > 0 {
		min, avg, max := utils.CalculateMinAvgMax(m.log.MemoryGCNotDeallocatedPerSecondGiB)
		m.log.MemoryGCNotDeallocatedStatsGiB = [3]float64{min, avg, max}
	}
}

func (pl *PerformanceLog) PrintMetrics() {
	logrus.Printf("********** Perf. Metrics for %s **********\n", pl.Description)
	logrus.Printf("Total Run time: %v sec\n", pl.StopTime.Sub(pl.StartTime).Seconds())
	logrus.Printf("CPU Usage Stats: min=%.2f%%, avg=%.2f%%, max=%.2f%%\n",
		pl.CpuUsageStats[0], pl.CpuUsageStats[1], pl.CpuUsageStats[2])
	logrus.Printf("Memory Allocated Stats: min=%.2f GiB, avg=%.2f GiB, max=%.2f GiB\n",
		pl.MemoryAllocatedStatsGiB[0], pl.MemoryAllocatedStatsGiB[1], pl.MemoryAllocatedStatsGiB[2])
	logrus.Printf("Memory InUse Stats: min=%.2f GiB, avg=%.2f GiB, max=%.2f GiB\n",
		pl.MemoryInUseStatsGiB[0], pl.MemoryInUseStatsGiB[1], pl.MemoryInUseStatsGiB[2])
	logrus.Printf("Memory GC Not Deallocated Stats: min=%.2f GiB, avg=%.2f GiB, max=%.2f GiB\n",
		pl.MemoryGCNotDeallocatedStatsGiB[0], pl.MemoryGCNotDeallocatedStatsGiB[1], pl.MemoryGCNotDeallocatedStatsGiB[2])
}

type PerfLogs []*PerformanceLog

func (pl PerfLogs) WritePerformanceLogsToCSV(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	startTime := time.Now()
	logrus.Infof("Writing the runtime performance logs to csv file located at path%s", path)

	// Define CSV headers
	headers := []string{
		"Description", "Runtime (s)",
		"CPU_Usage_Min", "CPU_Usage_Avg", "CPU_Usage_Max",
		"Mem_Allocated_Min (GiB)", "Mem_Allocated_Avg (GiB)", "Mem_Allocated_Max (GiB)",
		"Mem_InUse_Min (GiB)", "Mem_InUse_Avg (GiB)", "Mem_InUse_Max (GiB)",
		"Mem_GC_NotDeallocated_Min (GiB)", "Mem_GC_NotDeallocated_Avg (GiB)", "Mem_GC_NotDeallocated_Max (GiB)",
	}
	writer.Write(headers)

	// Write performance logs to CSV
	for _, log := range pl {
		record := []string{
			log.Description,
			strconv.FormatFloat(log.StopTime.Sub(log.StartTime).Seconds(), 'f', -1, 64),
			strconv.FormatFloat(log.CpuUsageStats[0], 'f', 2, 64),
			strconv.FormatFloat(log.CpuUsageStats[1], 'f', 2, 64),
			strconv.FormatFloat(log.CpuUsageStats[2], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryAllocatedStatsGiB[0], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryAllocatedStatsGiB[1], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryAllocatedStatsGiB[2], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryInUseStatsGiB[0], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryInUseStatsGiB[1], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryInUseStatsGiB[2], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryGCNotDeallocatedStatsGiB[0], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryGCNotDeallocatedStatsGiB[1], 'f', 2, 64),
			strconv.FormatFloat(log.MemoryGCNotDeallocatedStatsGiB[2], 'f', 2, 64),
		}
		writer.Write(record)
	}

	logrus.Infof("Finished writing to the csv file. Took %s", time.Since(startTime).String())
	return nil
}
